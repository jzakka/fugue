package crawler

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

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
