## 1. Environment and Dependencies

- [x] 1.1 Add OpenAI Go SDK to go.mod
- [x] 1.2 Add environment variable documentation for OPENAI_API_KEY and OPENAI_MODEL
- [x] 1.3 Update .env.example with AI configuration variables

## 2. AI Client Implementation

- [x] 2.1 Create AI client interface in internal/bot/ai/client.go
- [x] 2.2 Implement OpenAI client with codex chatGPT5.4 configuration
- [x] 2.3 Add timeout configuration (30 seconds)
- [x] 2.4 Add environment variable validation on initialization

## 3. Testing

- [x] 3.1 Add unit test for client initialization with valid credentials
- [x] 3.2 Add unit test for client initialization with missing credentials
- [x] 3.3 Add integration test with simple API call to verify connection (optional, requires API key) - SKIPPED (optional)
- [x] 3.4 Add test for timeout handling

## 4. Documentation

- [x] 4.1 Add setup instructions for OPENAI_API_KEY to README
- [x] 4.2 Document AI client interface for future use
- [x] 4.3 Add code comments explaining client configuration options
