package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIClient implements the Client interface using OpenAI's API.
type OpenAIClient struct {
	client  *openai.Client
	model   string
	timeout time.Duration
}

// Config holds configuration options for the OpenAI client.
type Config struct {
	// APIKey is the OpenAI API authentication key (required).
	APIKey string

	// Model is the model identifier (default: "gpt-5.4-codex").
	Model string

	// Timeout is the request timeout duration (default: 30 seconds).
	Timeout time.Duration
}

// NewOpenAIClient creates a new OpenAI client instance.
// Returns an error if APIKey is missing.
func NewOpenAIClient(cfg Config) (*OpenAIClient, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("OPENAI_API_KEY is required")
	}

	// Set defaults
	if cfg.Model == "" {
		cfg.Model = "gpt-4o" // Updated to real OpenAI model
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	client := openai.NewClient(cfg.APIKey)

	return &OpenAIClient{
		client:  client,
		model:   cfg.Model,
		timeout: cfg.Timeout,
	}, nil
}

// NewFromEnv creates a new AI client from environment variables.
// Supports two modes:
// - CLI mode (default): Uses local chatgpt command (AI_CLIENT_TYPE=cli or unset)
// - SDK mode: Uses OpenAI API (AI_CLIENT_TYPE=sdk, requires OPENAI_API_KEY)
func NewFromEnv() (Client, error) {
	clientType := os.Getenv("AI_CLIENT_TYPE")
	if clientType == "" {
		clientType = "cli" // Default to CLI mode
	}

	switch clientType {
	case "cli":
		return NewCLIClient(CLIConfig{}), nil

	case "sdk":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, errors.New("OPENAI_API_KEY environment variable is not set (required for SDK mode)")
		}

		model := os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4o" // Updated default model
		}

		return NewOpenAIClient(Config{
			APIKey: apiKey,
			Model:  model,
		})

	default:
		return nil, fmt.Errorf("invalid AI_CLIENT_TYPE: %s (must be 'cli' or 'sdk')", clientType)
	}
}

// Call sends a prompt to the AI model and returns the response.
func (c *OpenAIClient) Call(ctx context.Context, prompt string) (string, error) {
	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Create chat completion request
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.2,  // Deterministic output for code generation
		MaxTokens:   2000, // Reasonable limit for parsing scripts
	}

	// Make API call
	resp, err := c.client.CreateChatCompletion(ctxWithTimeout, req)
	if err != nil {
		return "", fmt.Errorf("AI API call failed: %w", err)
	}

	// Extract response
	if len(resp.Choices) == 0 {
		return "", errors.New("no response from AI model")
	}

	return resp.Choices[0].Message.Content, nil
}
