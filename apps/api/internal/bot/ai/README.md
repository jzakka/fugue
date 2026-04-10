# AI Client Documentation

## Overview

The AI client provides an interface for interacting with AI models (currently OpenAI's codex chatGPT5.4) for use in bot infrastructure, particularly for Pioneer's script generation.

## Interface

```go
type Client interface {
    Call(ctx context.Context, prompt string) (string, error)
}
```

The `Client` interface abstracts AI model interactions, allowing for future model swaps or multi-provider support.

## Implementation: OpenAI Client

### Initialization

#### CLI Mode (Default)

Uses a local `chatgpt` command (requires ChatGPT Plus subscription):

```go
import "github.com/chungsanghwa/fugue/apps/api/internal/bot/ai"

// Uses default chatgpt command
client, err := ai.NewFromEnv()
if err != nil {
    log.Fatal(err)
}

// Or explicitly create CLI client
client := ai.NewCLIClient(ai.CLIConfig{
    Command: "chatgpt",
    Args:    []string{"--model", "gpt-4"},
})
```

#### SDK Mode

Uses OpenAI API (requires API key):

Set `AI_CLIENT_TYPE=sdk` in environment, then:

```go
// From environment
client, err := ai.NewFromEnv()

// Or from config
    APIKey:  "your-api-key",
    Model:   "gpt-4o",
    Timeout: 30 * time.Second,
}

client, err := ai.NewOpenAIClient(cfg)
if err != nil {
    log.Fatal(err)
}
```

#### From Environment Variables

```go
// Reads AI_CLIENT_TYPE (defaults to "cli")
// CLI mode: just works if chatgpt command is available
// SDK mode: reads OPENAI_API_KEY and OPENAI_MODEL
client, err := ai.NewFromEnv()
if err != nil {
    log.Fatal(err)
}
```

### Configuration Options

| Option | Environment Variable | Default | Description |
|--------|---------------------|---------|-------------|
| Client Type | `AI_CLIENT_TYPE` | `cli` | `cli` (ChatGPT CLI) or `sdk` (OpenAI API) |
| **CLI Mode** | | | |
| Command | - | `chatgpt` | Local CLI command to execute |
| **SDK Mode** | | | |
| API Key | `OPENAI_API_KEY` | (required) | OpenAI API authentication key |
| Model | `OPENAI_MODEL` | `gpt-4o` | Model identifier |
| Timeout | - | 30 seconds | Request timeout duration |
| Temperature | - | 0.2 | Model temperature (deterministic for code) |
| Max Tokens | - | 2000 | Response token limit |

### Usage

```go
ctx := context.Background()
prompt := "Generate a parsing script for..."

response, err := client.Call(ctx, prompt)
if err != nil {
    // Handle error (network, timeout, API error, etc.)
    log.Printf("AI call failed: %v", err)
    return
}

// Use response
fmt.Println(response)
```

### Error Handling

The client returns errors for:
- **Missing API key**: Initialization fails with error
- **Network failures**: Connection issues
- **API timeouts**: Request exceeds timeout threshold
- **Authentication errors**: Invalid API key
- **Server errors**: OpenAI API issues

Example:
```go
_, err := client.Call(ctx, prompt)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // Handle timeout
    } else if strings.Contains(err.Error(), "authentication") {
        // Handle auth error
    } else {
        // Handle other errors
    }
}
```

### Context and Cancellation

The client respects context cancellation and timeouts:

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

response, err := client.Call(ctx, prompt)
```

## Future Use Cases

### Pioneer Script Generation

The AI client is designed for Pioneer to generate parsing scripts:

```go
// Pseudocode - not yet implemented
htmlContent := fetchPage(url)
prompt := buildPrompt(domain, nodeType, htmlContent)
script, err := aiClient.Call(ctx, prompt)
if err != nil {
    // Fallback or retry
}
saveScript(script)
```

### Other Bot Features

The interface can be used for:
- Content tagging and categorization
- Quality assessment
- Metadata extraction
- Natural language processing tasks

## Testing

### Unit Tests

Tests are located in `openai_test.go`:
- Client initialization (valid/invalid credentials)
- Environment variable loading
- Timeout behavior
- Context cancellation

Run tests:
```bash
cd apps/api/internal/bot/ai
go test -v
```

### Integration Testing

For integration testing with real API calls:

```bash
# Set API key
export OPENAI_API_KEY=your-real-key
export OPENAI_MODEL=gpt-5.4-codex

# Run integration tests (when added)
go test -v -tags=integration
```

## Notes

- **CLI Mode (Default)**: Requires `chatgpt` CLI installed and ChatGPT Plus subscription. No API costs.
- **SDK Mode**: Requires OpenAI API key. Costs ~$0.0006 per script generation.
- **Rate Limiting**: OpenAI enforces rate limits; the SDK handles retries. CLI mode has no rate limits.
- **Model Availability**: 
  - CLI: Uses whatever model your ChatGPT subscription includes
  - SDK: Ensure `gpt-4o` or your chosen model is available in your OpenAI account
- **Security**: Never commit API keys to version control; use environment variables or secret managers
