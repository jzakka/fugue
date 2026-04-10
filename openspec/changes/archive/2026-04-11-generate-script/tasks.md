## 1. Core Interfaces and Types

- [x] 1.1 Add `ValidationResult` type to `internal/bot/interfaces.go` with Success, ItemCount, ValidCount, ErrorMessage fields
- [x] 1.2 Update `AIClient.GenerateScript` signature to accept `maxIterations int` parameter
- [x] 1.3 Add JSON tags to `RawItem` struct in `internal/bot/source.go` for proper unmarshaling

## 2. AI Script Generation with Iteration

- [x] 2.1 Create `internal/bot/ai/cli.go` with CLIClient implementation
- [x] 2.2 Implement `GenerateScript` with iteration loop (1 to maxIterations)
- [x] 2.3 Implement `buildScriptPrompt` function accepting iteration, previousError, previousScriptPath
- [x] 2.4 Include extraction strategy guidance in prompt: JSON parsing priority, DOM scraping fallback
- [x] 2.5 Implement `extractJavaScript` to parse AI response and extract code blocks

## 3. Validation Logic

- [x] 3.1 Implement `validateItems` function with minRequired=10 threshold
- [x] 3.2 Count valid items: check non-empty media_url AND source_url
- [x] 3.3 Return ValidationResult with appropriate success status and counts
- [x] 3.4 Handle script execution errors in validation

## 4. Feedback Loop

- [x] 4.1 On validation failure, save script to `/tmp/fugue-failed-script-iter{N}-{domain}.js`
- [x] 4.2 Build error message: "Iteration N failed: {reason}. Extracted X items, Y valid (need at least 10)."
- [x] 4.3 Pass previousError and previousScriptPath to next iteration's buildScriptPrompt
- [x] 4.4 Include feedback in prompt with reference to failed script file

## 5. Script Execution

- [x] 5.1 Ensure `NodeExecutor` can execute generated Puppeteer scripts
- [x] 5.2 Pass HTML and URL as command-line arguments to script
- [x] 5.3 Parse JSON output from script into []RawItem
- [x] 5.4 Return execution errors for validation handling

## 6. Testing

- [x] 6.1 Remove `internal/bot/pixiv_test.go` (implementation test, not needed for production)
- [x] 6.2 Remove `internal/bot/soundcloud_test.go` (implementation test, not needed for production)
- [x] 6.3 Verify generated scripts extract 10+ items with required fields
- [x] 6.4 Test iteration loop by temporarily raising minRequired threshold
- [x] 6.5 Verify failed scripts are saved to /tmp with correct naming

## 7. Documentation

- [x] 7.1 Document AIClient interface usage in code comments
- [x] 7.2 Document validation criteria in ValidationResult type
- [x] 7.3 Add example of iterative generation flow in design doc or README

## Out of Scope (Future Work)

- Temporary file cleanup job for `/tmp/fugue-failed-script-*`
- Automatic script regeneration when website structure changes
- CAPTCHA/anti-bot handling during script execution
