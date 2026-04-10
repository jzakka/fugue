## Why

Pioneer will need AI capabilities to analyze web pages and generate parsing scripts. Before implementing the full pipeline, we need to set up the AI model client foundation. This change establishes the basic connection to codex chatGPT5.4, making it ready for future use.

## What Changes

- Add OpenAI Go SDK dependency
- Configure API credentials via environment variables
- Initialize AI model client (codex chatGPT5.4)
- Verify connection with test invocation

## Capabilities

### New Capabilities
- `bot`: Bot infrastructure including AI model client setup for future Pioneer script generation

### Modified Capabilities
<!-- No existing capabilities are being modified -->

## Impact

- **Code**: `apps/api/internal/bot/ai/` - AI client initialization
- **Configuration**: Environment variables for API key and model name
- **Dependencies**: OpenAI Go SDK
- **No DB changes**: This change only sets up the client, does not integrate with Pioneer workflow
