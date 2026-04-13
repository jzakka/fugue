package crawler

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

var selectorTargetTags = map[string]bool{
	"nav": true, "main": true, "aside": true, "footer": true,
	"header": true, "article": true, "section": true, "body": true,
}

func isSelectorTarget(tagName string) bool {
	return selectorTargetTags[tagName]
}

func divClassSelector(node *html.Node) (Selector, bool) {
	if node.Type != html.ElementNode || node.Data != "div" {
		return Selector{}, false
	}
	for _, attr := range node.Attr {
		if attr.Key == "class" && strings.TrimSpace(attr.Val) != "" {
			return Selector{TagName: "div", Class: attr.Val}, true
		}
	}
	return Selector{}, false
}

// ExtractLinksWithSelectors extracts links with their DOM ancestor selector paths.
func ExtractLinksWithSelectors(body io.Reader, baseURL string) ([]Link, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}

	var links []Link
	var visit func(*html.Node, []Selector)
	visit = func(n *html.Node, ancestors []Selector) {
		if n.Type == html.ElementNode {
			if n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						href := strings.TrimSpace(attr.Val)
						if href == "" {
							continue
						}
						if strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
							continue
						}
						absURL, err := makeAbsoluteURL(baseURL, href)
						if err != nil {
							continue
						}
						normalized, err := normalizeURL(absURL)
						if err != nil {
							continue
						}
						sels := make([]Selector, len(ancestors))
						copy(sels, ancestors)
						links = append(links, Link{URL: normalized, Selectors: sels})
						break
					}
				}
			} else if isSelectorTarget(n.Data) {
				ancestors = append(ancestors, Selector{TagName: n.Data})
			} else if sel, ok := divClassSelector(n); ok {
				ancestors = append(ancestors, sel)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c, ancestors)
		}
	}

	visit(doc, nil)
	return links, nil
}

// extractLinks extracts all links from HTML content.
// Returns absolute URLs, filtering out invalid or empty links.
func extractLinks(body io.Reader, baseURL string) ([]string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}

	var links []string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := strings.TrimSpace(attr.Val)

					// Skip empty href
					if href == "" {
						continue
					}

					// Skip javascript: and mailto: links
					if strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
						continue
					}

					// Convert to absolute URL
					absURL, err := makeAbsoluteURL(baseURL, href)
					if err != nil {
						continue
					}

					// Normalize the URL
					normalized, err := normalizeURL(absURL)
					if err != nil {
						continue
					}

					links = append(links, normalized)
					break
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}

	visit(doc)
	return links, nil
}
