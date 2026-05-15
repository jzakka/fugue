package bot

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
)

// HostRateSetter is the contract RobotsFilter needs from the scheduler side
// to propagate robots.txt Crawl-delay directives to the host token bucket.
// The scheduler-host-token-bucket capability's HostRateLimiter satisfies
// this interface via its SetHostRate(host, ratePerSec, burst) method.
type HostRateSetter interface {
	SetHostRate(host string, ratePerSec float64, burst int)
}

// robotsUserAgent is the preferred User-agent token used when matching
// robots.txt blocks. Falls back to "*" when no matching block exists.
const robotsUserAgent = "FugueBot"

// robotsCacheTTL controls how long a robots.txt fetch result is trusted
// before being re-fetched. fail-open entries share the same TTL to prevent
// retry storms against unhealthy hosts.
const robotsCacheTTL = 24 * time.Hour

// robotsFetchTimeout caps the robots.txt HTTP GET so a stalled host
// cannot block the filter chain.
const robotsFetchTimeout = 5 * time.Second

// robotsCacheEntry is the parsed robots.txt view for a single host.
type robotsCacheEntry struct {
	disallow   []string  // disallow path prefixes for the matched User-agent block
	crawlDelay *float64  // parsed Crawl-delay in seconds; nil if absent/invalid
	fetchedAt  time.Time // cache record time
	failOpen   bool      // true when fetch failed or returned 5xx; filter lets everything through
}

// RobotsFilter is a LinkFilter that consults robots.txt for each link's host.
// It caches parsed results in-memory per host with robotsCacheTTL, fails open
// on network / 5xx errors, and surfaces Crawl-delay directives through the
// optional HostRateSetter (typically the scheduler's HostRateLimiter).
//
// Concurrency: Filter may be called from multiple goroutines. A per-host
// single-flight serializes refresh attempts so that N concurrent misses for
// the same host produce exactly one HTTP GET and one HostRateSetter call
// per refresh — required by spec "Scenario: 캐시 TTL 내 중복 호출 방지".
type RobotsFilter struct {
	mu         sync.RWMutex
	cache      map[string]*robotsCacheEntry
	inflight   map[string]*inflightFetch // host → pending refresh coalescer
	httpClient *http.Client
	rateSetter HostRateSetter
	now        func() time.Time // injectable clock for tests
}

// inflightFetch lets duplicate refresh callers wait on the same fetch
// instead of launching their own HTTP GETs.
type inflightFetch struct {
	done   chan struct{}
	result *robotsCacheEntry
}

// NewRobotsFilter constructs a RobotsFilter. rateSetter may be nil, in which
// case Crawl-delay parsing still occurs but no scheduler call is made.
func NewRobotsFilter(rateSetter HostRateSetter) *RobotsFilter {
	return &RobotsFilter{
		cache:      make(map[string]*robotsCacheEntry),
		inflight:   make(map[string]*inflightFetch),
		httpClient: &http.Client{Timeout: robotsFetchTimeout},
		rateSetter: rateSetter,
		now:        time.Now,
	}
}

// RateSetter exposes the wired HostRateSetter for wiring-level assertions in
// bootstrap regression tests (spec: "Pioneer 부트스트랩은 RobotsFilter에
// HostRateLimiter를 wire한다"). Production code MUST NOT depend on this
// accessor; rate propagation flows through the internal refresh path only.
func (f *RobotsFilter) RateSetter() HostRateSetter {
	return f.rateSetter
}

// Filter returns only the links whose host either fails-open or is not
// Disallowed by the parsed robots.txt rules for the preferred User-agent.
func (f *RobotsFilter) Filter(links []crawler.Link) []crawler.Link {
	var out []crawler.Link
	for _, l := range links {
		u, err := url.Parse(l.URL)
		if err != nil || u.Host == "" {
			// Unparseable links can't be checked; drop them rather than
			// fail-open for an unrelated reason.
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == "" {
			continue
		}
		entry := f.getOrFetch(host)
		if entry.failOpen {
			out = append(out, l)
			continue
		}
		if isDisallowed(u.Path, entry.disallow) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// getOrFetch returns a cache entry for host, refreshing it on miss or TTL
// expiry. Concurrent refreshes for the same host are coalesced via a
// per-host inflight channel so exactly one HTTP GET and one HostRateSetter
// call occur per refresh cycle.
func (f *RobotsFilter) getOrFetch(host string) *robotsCacheEntry {
	f.mu.RLock()
	entry, ok := f.cache[host]
	f.mu.RUnlock()
	if ok && f.now().Sub(entry.fetchedAt) <= robotsCacheTTL {
		return entry
	}
	return f.refresh(host)
}

// refresh coalesces duplicate refresh calls per host. The first caller owns
// the HTTP fetch; subsequent callers wait for the same result.
func (f *RobotsFilter) refresh(host string) *robotsCacheEntry {
	f.mu.Lock()
	// Re-check the cache under the write lock: another goroutine may have
	// just completed a refresh while we were waiting on the lock.
	if entry, ok := f.cache[host]; ok && f.now().Sub(entry.fetchedAt) <= robotsCacheTTL {
		f.mu.Unlock()
		return entry
	}
	if pending, ok := f.inflight[host]; ok {
		// A refresh is already running; wait on its completion instead of
		// launching a duplicate HTTP GET.
		f.mu.Unlock()
		<-pending.done
		return pending.result
	}
	pending := &inflightFetch{done: make(chan struct{})}
	f.inflight[host] = pending
	f.mu.Unlock()

	// Outside the lock: network fetch (up to robotsFetchTimeout).
	entry := f.fetch(host)

	f.mu.Lock()
	f.cache[host] = entry
	delete(f.inflight, host)
	f.mu.Unlock()

	pending.result = entry
	close(pending.done)

	// Surface Crawl-delay to the scheduler only from the refresh path
	// (the single-flight winner). Waiters on pending.done return the same
	// result but do not re-invoke SetHostRate, and cache-hit lookups within
	// TTL skip refresh entirely — together guaranteeing the spec's
	// "Scenario: 캐시 TTL 내 중복 호출 방지".
	if entry.crawlDelay != nil && *entry.crawlDelay > 0 && f.rateSetter != nil {
		rate := 1.0 / *entry.crawlDelay
		f.rateSetter.SetHostRate(host, rate, 1)
	}
	return entry
}

// fetch performs the HTTP GET and parse. It never returns an error; network
// or 5xx failures are represented as failOpen entries so the filter defers
// to "allow everything" instead of stalling the crawl.
func (f *RobotsFilter) fetch(host string) *robotsCacheEntry {
	now := f.now()
	resp, err := f.httpClient.Get("https://" + host + "/robots.txt")
	if err != nil {
		return &robotsCacheEntry{fetchedAt: now, failOpen: true}
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		// 404 = no robots.txt published = no restrictions (empty rule set).
		return &robotsCacheEntry{fetchedAt: now}
	case resp.StatusCode >= 500:
		// 5xx = server failure: fail-open so a flaky origin cannot stall
		// the crawler. TTL-cached to suppress retry storms.
		return &robotsCacheEntry{fetchedAt: now, failOpen: true}
	case resp.StatusCode != http.StatusOK:
		// Non-2xx other than 404/5xx (auth walls, 3xx that do not
		// terminate, rate limits, ...) are conservatively treated as
		// "no rules published" rather than fail-open — the origin
		// neither published rules nor encountered a transient failure,
		// so do not widen policy.
		return &robotsCacheEntry{fetchedAt: now}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &robotsCacheEntry{fetchedAt: now, failOpen: true}
	}
	disallow, crawlDelay := parseRobotsTxt(string(body), robotsUserAgent)
	return &robotsCacheEntry{
		disallow:   disallow,
		crawlDelay: crawlDelay,
		fetchedAt:  now,
	}
}

// isDisallowed reports whether urlPath is matched by any of the Disallow
// patterns. Pattern matching is prefix-based per RFC 9309's simplest form;
// wildcards (`*`, `$`) are not honored here because the spec scope is
// limited to plain Disallow prefixes.
func isDisallowed(urlPath string, patterns []string) bool {
	if urlPath == "" {
		urlPath = "/"
	}
	for _, p := range patterns {
		if p == "" {
			// "Disallow:" (empty) means "allow all" for this UA block.
			continue
		}
		if strings.HasPrefix(urlPath, p) {
			return true
		}
	}
	return false
}

// parseRobotsTxt returns the Disallow prefixes and Crawl-delay (if any) for
// the preferredUA block. If no preferredUA block exists the "*" block is
// used. Blocks are not merged.
func parseRobotsTxt(body, preferredUA string) ([]string, *float64) {
	// Two parallel collectors: preferred and wildcard. We emit the
	// preferred block when it contains at least one User-agent line
	// matching preferredUA; otherwise we fall back to the wildcard block.
	type block struct {
		disallow   []string
		crawlDelay *float64
		present    bool
	}
	var preferred, wildcard block

	// currentBlocks tracks which block(s) the current group of
	// `User-agent:` lines is writing into; consecutive UA lines form a
	// group per RFC 9309.
	var currentTargets []*block
	prevDirectiveWasUA := false

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			// Blank line ends the current group per RFC 9309; the next
			// User-agent line starts a new group.
			currentTargets = nil
			prevDirectiveWasUA = false
			continue
		}
		name, value, ok := splitDirective(line)
		if !ok {
			continue
		}
		switch strings.ToLower(name) {
		case "user-agent":
			if !prevDirectiveWasUA {
				currentTargets = nil
			}
			ua := strings.ToLower(strings.TrimSpace(value))
			var target *block
			if ua == strings.ToLower(preferredUA) {
				preferred.present = true
				target = &preferred
			} else if ua == "*" {
				wildcard.present = true
				target = &wildcard
			}
			if target != nil {
				// De-duplicate: a UA group may repeat the same agent name
				// (e.g. `User-agent: FugueBot\nUser-agent: FugueBot\n`)
				// which would otherwise cause Disallow lines to be recorded
				// twice for the same block.
				alreadyTargeted := false
				for _, t := range currentTargets {
					if t == target {
						alreadyTargeted = true
						break
					}
				}
				if !alreadyTargeted {
					currentTargets = append(currentTargets, target)
				}
			}
			prevDirectiveWasUA = true
		case "disallow":
			for _, t := range currentTargets {
				t.disallow = append(t.disallow, strings.TrimSpace(value))
			}
			prevDirectiveWasUA = false
		case "crawl-delay":
			if d, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && d > 0 {
				dv := d
				for _, t := range currentTargets {
					t.crawlDelay = &dv
				}
			}
			prevDirectiveWasUA = false
		default:
			prevDirectiveWasUA = false
		}
	}

	if preferred.present {
		return preferred.disallow, preferred.crawlDelay
	}
	if wildcard.present {
		return wildcard.disallow, wildcard.crawlDelay
	}
	return nil, nil
}

// stripComment removes a trailing `#...` comment if present.
func stripComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		return line[:i]
	}
	return line
}

// splitDirective splits "Name: value" into (name, value). It returns
// (_, _, false) when the line lacks a colon.
func splitDirective(line string) (string, string, bool) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}
