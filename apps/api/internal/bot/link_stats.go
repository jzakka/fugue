package bot

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// ComputeLinkStats walks the HTML once and returns the link/word counts the
// classifier consumes. Words are counted by whitespace-separated tokens of
// visible text; <script>/<style>/<noscript> contents are ignored.
func ComputeLinkStats(htmlBytes []byte) LinkStats {
	if len(htmlBytes) == 0 {
		return LinkStats{}
	}
	root, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return LinkStats{}
	}

	var stats LinkStats
	var b strings.Builder

	var walk func(n *html.Node, skip bool)
	walk = func(n *html.Node, skip bool) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			switch tag {
			case "script", "style", "noscript":
				return
			case "a":
				stats.Links++
			}
		}
		if !skip && n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skip)
		}
	}
	walk(root, false)

	stats.Words = countWords(b.String())
	return stats
}

func countWords(s string) int {
	return len(strings.Fields(s))
}
