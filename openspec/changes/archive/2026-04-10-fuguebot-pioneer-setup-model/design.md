## Context

Bot infrastructure needs an AI model client for future Pioneer functionality. According to `docs/bot-architecture.md`, Pioneer will use AI to generate parsing scripts, but that integration is a future task. This change only sets up the AI client foundation.

Current state:
- No AI client exists
- No API credentials configured
- Bot domain not yet created in openspec/specs/

## Goals / Non-Goals

**Goals:**
- Add OpenAI Go SDK to the project
- Initialize AI client with codex chatGPT5.4 configuration
- Verify the client can connect and make basic API calls
- Provide reusable AI client interface for future use

**Non-Goals:**
- Pioneer workflow integration (future task)
- Prompt engineering for script generation (future task)
- Cost tracking and DB persistence (future task)
- Supporting multiple model providers (chatGPT5.4 only)
- Fine-tuning the model (use as-is)

## Decisions

### Decision 1: Use OpenAI SDK for chatGPT5.4
**Rationale**: Official SDK provides robust error handling, retries, and streaming support. Codex chatGPT5.4 is accessed via OpenAI's API.

**Alternatives considered**:
- Raw HTTP client: More control but requires manual retry logic and error handling
- Generic LLM abstraction: Over-engineering for single-model use case

### Decision 2: Store API credentials in environment variables
**Rationale**: Standard practice for secrets management in containerized environments. Works with Kubernetes secrets.

**Environment variables**:
- `OPENAI_API_KEY`: API authentication key
- `OPENAI_MODEL`: Model identifier (default: "gpt-5.4-codex" or "chatgpt-5.4")
- `OPENAI_MAX_TOKENS`: Response token limit (default: 2000)
- `OPENAI_TEMPERATURE`: Model temperature (default: 0.2 for deterministic output)

**Alternatives considered**:
- Config file: Less secure, harder to rotate credentials
- Vault integration: Overkill for MVP

### Decision 3: Basic timeout and retry strategy
**Rationale**: AI API calls can be slow or fail. Set conservative timeouts and retry transient failures.

**Configuration**:
- Timeout: 30 seconds per request
- Retries: SDK default retry behavior
- Error handling: Return errors to caller for now

**Alternatives considered**:
- Custom retry logic: Not needed, SDK handles this
- Circuit breaker pattern: Overkill for initial setup

## Risks / Trade-offs

**[Risk] Model API changes or deprecation**
→ Mitigation: Abstract AI client behind interface, making model swaps easier

**[Risk] API costs during testing**
→ Mitigation: Use minimal test calls, document cost expectations

**[Trade-off] Single model vendor lock-in**
→ Acceptable for MVP. Interface abstraction allows future multi-model support.

**[Trade-off] Environment variables for config vs. database**
→ Environment is simpler for setup. Future integration can add DB configuration if needed.

## Migration Plan

Not applicable - this is a new feature, no migration needed.

## Open Questions

None - all technical decisions are finalized.
