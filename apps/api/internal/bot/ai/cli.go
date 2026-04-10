package ai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
)

// CLIClient implements AI client using command-line tool (codex)
type CLIClient struct {
	command string
	args    []string
}

// CLIConfig holds configuration for CLI-based AI client
type CLIConfig struct {
	Command string   // e.g., "codex"
	Args    []string // e.g., []string{"exec"}
}

// NewCLIClient creates a new CLI-based AI client
func NewCLIClient(cfg CLIConfig) *CLIClient {
	return &CLIClient{
		command: cfg.Command,
		args:    cfg.Args,
	}
}

// Call invokes the CLI tool with the given prompt
func (c *CLIClient) Call(ctx context.Context, prompt string) (string, error) {
	args := append(c.args, prompt)
	cmd := exec.CommandContext(ctx, c.command, args...)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cli command failed: %w (output: %s)", err, string(output))
	}
	
	return string(output), nil
}

// GenerateScript generates a parsing script with iterative improvement
func (c *CLIClient) GenerateScript(ctx context.Context, req bot.ScriptRequest, maxIterations int) (bot.ScriptResponse, error) {
	if maxIterations <= 0 {
		maxIterations = 1 // At least one attempt
	}

	var lastError string
	var lastScriptPath string
	
	// Track best attempt across all iterations
	var bestScript string
	var bestValidCount int
	var bestIteration int
	
	for iteration := 1; iteration <= maxIterations; iteration++ {
		// Build prompt (include feedback from previous iteration)
		prompt := buildScriptPrompt(req, iteration, lastError, lastScriptPath)

		// Call AI
		response, err := c.Call(ctx, prompt)
		if err != nil {
			return bot.ScriptResponse{}, fmt.Errorf("AI call failed (iteration %d): %w", iteration, err)
		}

		// Extract JavaScript code
		scriptCode := extractJavaScript(response)
		
		// Validate by executing the script
		executor := bot.NewNodeExecutorWithDir(".")
		items, execErr := executor.Execute(ctx, scriptCode, req.HTML, req.URL)
		
		validation := validateItems(items, execErr)
		
		// Track best attempt
		if validation.ValidCount > bestValidCount {
			bestScript = scriptCode
			bestValidCount = validation.ValidCount
			bestIteration = iteration
		}
		
		if validation.Success {
			// Success! Return the script
			return bot.ScriptResponse{
				ScriptCode: scriptCode,
				CostUSD:    0.0,
				Model:      fmt.Sprintf("chatgpt-cli (iteration %d)", iteration),
			}, nil
		}
		
		// Failed - save script to tmp for next iteration
		tmpPath := fmt.Sprintf("/tmp/fugue-failed-script-iter%d-%s.js", iteration, req.Domain)
		if err := os.WriteFile(tmpPath, []byte(scriptCode), 0644); err != nil {
			// Non-fatal: log but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to save debug script to %s: %v\n", tmpPath, err)
		}
		
		lastScriptPath = tmpPath
		lastError = fmt.Sprintf("Iteration %d failed: %s. Extracted %d items, %d valid (need at least 10).",
			iteration, validation.ErrorMessage, validation.ItemCount, validation.ValidCount)
	}

	// All iterations failed - return best attempt if it extracted any items
	if bestValidCount > 0 {
		return bot.ScriptResponse{
			ScriptCode: bestScript,
			CostUSD:    0.0,
			Model:      fmt.Sprintf("chatgpt-cli (best of %d iterations: %d valid items from iteration %d)", maxIterations, bestValidCount, bestIteration),
		}, nil
	}

	return bot.ScriptResponse{}, fmt.Errorf("script generation failed: best attempt extracted 0 valid items after %d iterations", maxIterations)
}

func validateItems(items []bot.RawItem, execErr error) bot.ValidationResult {
	if execErr != nil {
		return bot.ValidationResult{
			Success:      false,
			ItemCount:    0,
			ValidCount:   0,
			ErrorMessage: fmt.Sprintf("Script execution error: %v", execErr),
		}
	}
	
	validCount := 0
	for _, item := range items {
		if item.MediaURL != "" && item.SourceURL != "" {
			validCount++
		}
	}
	
	// Success criteria: at least 10 valid items (to ensure good coverage)
	minRequired := 10
	
	if validCount < minRequired {
		return bot.ValidationResult{
			Success:      false,
			ItemCount:    len(items),
			ValidCount:   validCount,
			ErrorMessage: fmt.Sprintf("Only %d valid items (need at least %d)", validCount, minRequired),
		}
	}
	
	return bot.ValidationResult{
		Success:      true,
		ItemCount:    len(items),
		ValidCount:   validCount,
		ErrorMessage: "",
	}
}

func buildScriptPrompt(req bot.ScriptRequest, iteration int, previousError string, previousScriptPath string) string {
	html := req.HTML
	scriptTags := extractScriptTags(html)
	
	htmlSample := ""
	if len(scriptTags) > 0 {
		htmlSample = html[:min(5000, len(html))] + "\n\n[... script tags ...]\n" + scriptTags + "\n\n[... rest ...]"
		if len(htmlSample) > 30000 {
			htmlSample = htmlSample[:30000] + "\n...[truncated]"
		}
	} else {
		if len(html) > 20000 {
			htmlSample = html[:20000] + "\n...[truncated]"
		} else {
			htmlSample = html
		}
	}
	
	feedbackSection := ""
	if iteration > 1 && previousError != "" {
		feedbackSection = fmt.Sprintf(`
PREVIOUS ATTEMPT (Iteration %d) FAILED:
%s

Failed script saved to: %s
(You can reference this file to avoid repeating the same mistakes)

IMPORTANT: Try a DIFFERENT approach. Analyze what went wrong and use a different extraction strategy.
`, iteration-1, previousError, previousScriptPath)
	}

	return fmt.Sprintf(`Generate a Node.js Puppeteer script to extract items from %s.

URL: %s
Iteration: %d/%s

REQUIREMENTS:
1. Use Puppeteer (headless browser) to handle dynamic content
2. Navigate to the URL and wait for content to load
3. Extract items with these fields:
   - media_url: Direct URL to media file or thumbnail (REQUIRED, non-empty)
   - source_url: Link to item detail page (REQUIRED, non-empty)
   - title: Item title (optional, can be empty)
   - media_type: "image" for visual art, "audio" for music
4. Script execution: node script.js "" "<url>"
   - process.argv[3] = page URL

EXTRACTION STRATEGY (priority order):
1. FIRST: Try parsing JSON from <script id="__NEXT_DATA__"> or window.* globals
   - Walk the JSON tree to find all artwork/track objects
   - This usually gives 50-100+ items
2. FALLBACK: If JSON parsing fails, use DOM scraping
   - Find links with images
   - This usually gives 5-20 items

SUCCESS CRITERIA:
- Extract at least 10 items (prefer JSON parsing for better coverage)
- Every item MUST have both media_url and source_url (non-empty)
- Skip items missing either field: if (!mediaUrl || !sourceUrl) continue

SCRIPT STRUCTURE:
- Launch Puppeteer with headless mode
- Navigate to URL
- Use page.evaluate() to extract data from DOM
- Output JSON array to stdout: console.log(JSON.stringify(items))
- Close browser in finally block
- ES5 syntax only (var, function)

SECURITY CONSTRAINTS:
- DO NOT spawn child processes or execute system commands
- DO NOT use require() to load external packages beyond puppeteer
- DO NOT access filesystem (fs module)
- DO NOT access environment variables (process.env)
- DO NOT make network requests beyond the target URL
- Set timeout: page.goto(url, {timeout: 60000})
- Close browser in finally block to prevent resource leaks

HTML Sample:
%s`, req.Domain, req.URL, iteration, feedbackSection, htmlSample)
}

func extractScriptTags(html string) string {
	scriptRe := regexp.MustCompile(`(?i)<script[^>]*>([\s\S]*?)</script>`)
	matches := scriptRe.FindAllStringSubmatch(html, -1)
	
	result := ""
	for i, match := range matches {
		if i >= 10 || len(result) > 15000 {
			break
		}
		if len(match) > 1 {
			content := match[1]
			if len(content) > 100 && (strings.Contains(content, "{") || strings.Contains(content, "[")) {
				result += "<script>\n" + content + "\n</script>\n\n"
			}
		}
	}
	return result
}

func extractJavaScript(response string) string {
	lines := strings.Split(response, "\n")
	
	// Try markdown code block
	inCodeBlock := false
	codeBlockContent := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```javascript" || trimmed == "```js" || trimmed == "```" {
			if !inCodeBlock {
				inCodeBlock = true
				codeBlockContent = []string{}
			} else {
				break
			}
			continue
		}
		if inCodeBlock {
			codeBlockContent = append(codeBlockContent, line)
		}
	}
	
	if len(codeBlockContent) > 0 {
		return strings.TrimSpace(strings.Join(codeBlockContent, "\n"))
	}
	
	// Fallback: find code start
	var cleanedLines []string
	foundStart := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if strings.Contains(line, "HTML:") || strings.Contains(line, "HTML Sample") {
			break
		}
		
		if !foundStart {
			if strings.HasPrefix(trimmed, "var puppeteer") || 
			   strings.HasPrefix(trimmed, "var cheerio") {
				foundStart = true
			}
		}
		
		if foundStart {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(cleanedLines, "\n"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
