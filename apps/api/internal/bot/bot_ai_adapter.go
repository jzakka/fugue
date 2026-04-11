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

Generate a JavaScript script that extracts relevant items from this HTML.
Return a JSON array of items with fields: title, description, mediaURL, sourceURL, mediaType.

Output only valid JSON with the script code.`, req.Domain, req.URL, req.NodeType, truncateHTML(req.HTML, 5000))

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

// truncateHTML truncates HTML content to a maximum length
func truncateHTML(html string, maxLen int) string {
	if len(html) <= maxLen {
		return html
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
