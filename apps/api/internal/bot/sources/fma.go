package sources

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
)

const (
	fmaBaseURL = "https://freemusicarchive.org"
)

// FMA implements the bot.Source interface for Free Music Archive using Colly.
type FMA struct {
	seedURLs []string
}

// NewFMA creates a new Free Music Archive source.
// seedURLs should be paths like "/music/Electronic", "/music/Rock", etc.
func NewFMA(seedURLs []string) *FMA {
	if len(seedURLs) == 0 {
		seedURLs = []string{"/music"}
	}
	return &FMA{seedURLs: seedURLs}
}

func (f *FMA) Name() string { return "fma" }

// Crawl scrapes track listings from Free Music Archive.
func (f *FMA) Crawl(ctx context.Context) ([]bot.RawItem, error) {
	var mu sync.Mutex
	var items []bot.RawItem

	c := colly.NewCollector(
		colly.AllowedDomains("freemusicarchive.org", "www.freemusicarchive.org"),
		colly.UserAgent("FugueBot/1.0 (+https://fugue.app/bot)"),
	)

	// Respect robots.txt (enabled by default in Colly)

	// Rate limiting: 1 request per second
	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*freemusicarchive.org*",
		Parallelism: 1,
		Delay:       time.Second,
	}); err != nil {
		log.Printf("fma: failed to set rate limit: %v", err)
	}

	// Each track row in the music listing page
	c.OnHTML(".play-item", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.ChildText(".ptxt-track a"))
		artist := strings.TrimSpace(e.ChildText(".ptxt-artist a"))
		trackURL := e.ChildAttr(".ptxt-track a", "href")
		downloadURL := e.ChildAttr("a.icn-arrow", "href")

		if title == "" || downloadURL == "" {
			return
		}

		// Build full URLs
		if !strings.HasPrefix(trackURL, "http") {
			trackURL = fmaBaseURL + trackURL
		}
		if !strings.HasPrefix(downloadURL, "http") {
			downloadURL = fmaBaseURL + downloadURL
		}

		desc := ""
		if artist != "" {
			desc = fmt.Sprintf("by %s (Free Music Archive, CC License)", artist)
		}

		mu.Lock()
		items = append(items, bot.RawItem{
			Title:       title,
			Description: desc,
			MediaURL:    downloadURL,
			SourceURL:   trackURL,
			MediaType:   "audio",
		})
		mu.Unlock()
	})

	// Alternative: some FMA pages use a different structure
	c.OnHTML(".bcrumb-track-item", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.ChildText(".track-title a"))
		artist := strings.TrimSpace(e.ChildText(".track-artist a"))
		trackURL := e.ChildAttr(".track-title a", "href")
		downloadURL := e.ChildAttr("a.download-btn", "href")

		if title == "" || downloadURL == "" {
			return
		}

		if !strings.HasPrefix(trackURL, "http") {
			trackURL = fmaBaseURL + trackURL
		}
		if !strings.HasPrefix(downloadURL, "http") {
			downloadURL = fmaBaseURL + downloadURL
		}

		desc := ""
		if artist != "" {
			desc = fmt.Sprintf("by %s (Free Music Archive, CC License)", artist)
		}

		mu.Lock()
		items = append(items, bot.RawItem{
			Title:       title,
			Description: desc,
			MediaURL:    downloadURL,
			SourceURL:   trackURL,
			MediaType:   "audio",
		})
		mu.Unlock()
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("fma: error crawling %s: %v", r.Request.URL, err)
	})

	// Visit each seed URL
	for _, seedPath := range f.seedURLs {
		url := seedPath
		if !strings.HasPrefix(url, "http") {
			url = fmaBaseURL + seedPath
		}

		log.Printf("fma: visiting %s", url)
		if err := c.Visit(url); err != nil {
			log.Printf("fma: failed to visit %s: %v", url, err)
		}
	}

	c.Wait()

	log.Printf("fma: crawled %d tracks", len(items))
	return items, nil
}
