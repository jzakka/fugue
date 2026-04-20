package bot

import (
	"context"
	"strings"
	"sync"
)

// PerSiteAdapter is the interface a per-site extractor implements when it
// wants to override the GenericExtractor for a specific domain.
type PerSiteAdapter interface {
	// Domain returns the domain this adapter handles. Wildcard subdomains
	// are expressed as "*.example.com".
	Domain() string

	// Name returns a stable identifier persisted to og_data.extractor.
	// For ScriptAdapter this is "script:<site_id>"; other adapters return
	// their own name.
	Name() string

	// Extract turns HTML into a PinDocument. fetchURL is the URL the
	// Harvester actually fetched.
	Extract(ctx context.Context, htmlBytes []byte, fetchURL string) (PinDocument, error)
}

// AdapterRegistry resolves a domain to a PerSiteAdapter.
type AdapterRegistry interface {
	Resolve(domain string) (PerSiteAdapter, bool)
	Register(adapter PerSiteAdapter)
}

// InMemoryAdapterRegistry is the default in-process implementation.
//
// Lookup precedence:
//
//  1. Exact domain match (e.g. "pixiv.net" registration matches
//     "pixiv.net" lookups exactly).
//  2. Wildcard subdomain match (e.g. "*.example.com" registration
//     matches "foo.example.com" or "a.b.example.com").
//
// A registered "*.example.com" does NOT match the bare "example.com" — if a
// site needs both, register both explicitly.
type InMemoryAdapterRegistry struct {
	mu       sync.RWMutex
	exact    map[string]PerSiteAdapter
	wildcard map[string]PerSiteAdapter // suffix → adapter, key is "example.com"
}

// NewInMemoryAdapterRegistry returns an empty in-memory registry.
func NewInMemoryAdapterRegistry() *InMemoryAdapterRegistry {
	return &InMemoryAdapterRegistry{
		exact:    map[string]PerSiteAdapter{},
		wildcard: map[string]PerSiteAdapter{},
	}
}

// Register adds an adapter. A "*.example.com" Domain() is interpreted as a
// wildcard subdomain match. Registering the same key twice replaces the
// previous adapter.
func (r *InMemoryAdapterRegistry) Register(adapter PerSiteAdapter) {
	if adapter == nil {
		return
	}
	domain := strings.ToLower(strings.TrimSpace(adapter.Domain()))
	if domain == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.HasPrefix(domain, "*.") {
		r.wildcard[strings.TrimPrefix(domain, "*.")] = adapter
		return
	}
	r.exact[domain] = adapter
}

// Resolve returns the adapter for the given domain, if any. Domain is
// matched case-insensitively.
func (r *InMemoryAdapterRegistry) Resolve(domain string) (PerSiteAdapter, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if a, ok := r.exact[domain]; ok {
		return a, true
	}
	// Wildcard match: longest suffix wins.
	var best PerSiteAdapter
	bestLen := -1
	for suffix, a := range r.wildcard {
		if domain == suffix {
			// "*.example.com" must NOT match the bare "example.com".
			continue
		}
		if strings.HasSuffix(domain, "."+suffix) && len(suffix) > bestLen {
			best = a
			bestLen = len(suffix)
		}
	}
	if best != nil {
		return best, true
	}
	return nil, false
}

// FillExtractorIdentity ensures doc.OGData.Extractor is set. The Harvester
// calls this after Extract so the field is consistent regardless of which
// extractor produced the document.
func FillExtractorIdentity(doc *PinDocument, name string) {
	if doc == nil || doc.OGData.Extractor != "" {
		return
	}
	doc.OGData.Extractor = name
}
