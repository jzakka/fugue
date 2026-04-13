package bot

import (
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
)

// LinkFilter defines the contract for filtering crawled links.
// Implementations receive a slice of links and return only those that pass the filter criteria.
type LinkFilter interface {
	Filter(links []crawler.Link) []crawler.Link
}

// FilterChain applies multiple LinkFilter instances in sequence.
// Each filter's output becomes the next filter's input.
type FilterChain struct {
	filters []LinkFilter
}

// NewFilterChain creates a FilterChain with the given filters applied in order.
func NewFilterChain(filters ...LinkFilter) *FilterChain {
	return &FilterChain{filters: filters}
}

// Apply runs all registered filters sequentially on the input links.
// Returns nil for nil input. Returns the input unchanged if no filters are registered.
func (c *FilterChain) Apply(links []crawler.Link) []crawler.Link {
	if links == nil {
		return nil
	}
	for _, f := range c.filters {
		if f == nil {
			continue
		}
		links = f.Filter(links)
	}
	return links
}
