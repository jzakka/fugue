package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/ai"
)

// AIClientAdapter adapts the ai.Client interface to the bot.AIClient interface
type AIClientAdapter struct {
	client ai.Client
}

// NewAIClientAdapter creates a new adapter that wraps an ai.Client
func NewAIClientAdapter(client ai.Client) *AIClientAdapter {
	return &AIClientAdapter{
		client: client,
	}
}

// GenerateScript implements the bot.AIClient interface
func (a *AIClientAdapter) GenerateScript(ctx context.Context, req ScriptRequest) (ScriptResponse, error) {
	// Build the prompt for the AI
	prompt := fmt.Sprintf(`Generate a JavaScript parsing script for the following web page:

Domain: %s
URL: %s
Node Type: %s

HTML Content:
%s

Generate a JavaScript IIFE that extracts relevant items from this HTML and returns an array.
Each item must have fields: title, description, mediaURL, sourceURL, mediaType.

SPA DETECTION:
If the HTML contains signs of a Single Page Application (e.g. <script id="__NEXT_DATA__">,
data-reactroot, __NUXT__, empty <div id="root">, or very few <a> tags with no real content),
then the actual data is NOT in the DOM elements. Instead:
1. First check for <script id="__NEXT_DATA__" type="application/json"> and parse its JSON content.
   Access it via: document.querySelector('script#__NEXT_DATA__').textContent, then JSON.parse it.
2. Extract items from the parsed JSON structure (e.g. props.pageProps or embedded state).
3. If __NEXT_DATA__ doesn't contain the needed data, note that this page requires API-based extraction.

Do NOT try to parse <a> tags or DOM elements if the page is a SPA with no server-rendered content.

IMPORTANT: Output ONLY the raw JavaScript code. Do NOT wrap it in JSON. Do NOT use markdown code fences.
The script should be a self-executing function like: (function(){ ... return items; })()`, req.Domain, req.URL, req.NodeType, truncateHTML(req.HTML, 5000))

	// Call the AI client
	response, err := a.client.Call(ctx, prompt)
	if err != nil {
		return ScriptResponse{}, fmt.Errorf("AI client call failed: %w", err)
	}

	// Extract script code from response
	scriptCode := extractScriptFromResponse(response)

	return ScriptResponse{
		ScriptCode: scriptCode,
		CostUSD:    0.01, // Approximate cost
		Model:      "gpt-4o",
	}, nil
}

// truncateHTML truncates HTML content to a maximum length.
// If the HTML contains SPA markers like __NEXT_DATA__, it includes
// both the head portion and the SPA data section for AI analysis.
func truncateHTML(html string, maxLen int) string {
	if len(html) <= maxLen {
		return html
	}

	// Check for SPA data markers
	spaMarkers := []string{
		`<script id="__NEXT_DATA__"`,
		`<script id="__NUXT__"`,
		`window.__INITIAL_STATE__`,
	}

	for _, marker := range spaMarkers {
		idx := strings.Index(html, marker)
		if idx < 0 {
			continue
		}
		// Found SPA data — include head (2000 chars) + SPA section
		headPart := html[:min(2000, len(html))]

		// Extract SPA script section (up to closing </script>)
		endIdx := strings.Index(html[idx:], "</script>")
		spaEnd := idx + endIdx + len("</script>")
		if endIdx < 0 {
			spaEnd = min(idx+maxLen, len(html))
		}
		spaPart := html[idx:min(spaEnd, len(html))]

		// Truncate SPA part if too large (keep first portion for structure)
		if len(spaPart) > maxLen {
			spaPart = spaPart[:maxLen] + "...(truncated)"
		}

		return headPart + "\n...(SPA detected, skipping to data section)...\n" + spaPart
	}

	return html[:maxLen] + "..."
}

// extractScriptFromResponse extracts JavaScript code from the AI response
func extractScriptFromResponse(response string) string {
	// Try to extract code from markdown code blocks
	if strings.Contains(response, "```") {
		parts := strings.Split(response, "```")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			// Skip language identifiers
			if strings.HasPrefix(part, "javascript") || strings.HasPrefix(part, "js") {
				lines := strings.Split(part, "\n")
				if len(lines) > 1 {
					return strings.Join(lines[1:], "\n")
				}
			}
			// If it looks like code (starts with var, function, const, etc.)
			if strings.HasPrefix(part, "function") || strings.HasPrefix(part, "const") ||
				strings.HasPrefix(part, "var") || strings.HasPrefix(part, "let") {
				return part
			}
		}
	}

	// Otherwise return the response as-is
	return response
}
