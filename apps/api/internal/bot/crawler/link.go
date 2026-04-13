// link.go defines structured types for links extracted from HTML documents.
// These types carry DOM context alongside the URL, enabling selector-based filtering.
package crawler

// Selector represents a single DOM element's identifying attributes (tag name, ID, class).
type Selector struct {
	TagName string
	ID      string
	Class   string
}

// Link represents a hyperlink extracted from an HTML document.
// Selectors holds the ancestor path from the DOM root down to the <a> element,
// allowing filters to make decisions based on where a link appears in the page structure.
type Link struct {
	URL       string
	Selectors []Selector
}
