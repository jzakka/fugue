package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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
// Automatically selects implementation based on ENV:
// - ENV=local (or unset): Uses codex CLI subprocess (default for local development)
// - ENV=production/staging: Uses OpenAI SDK (requires OPENAI_API_KEY)
func NewFromEnv() (Client, error) {
	env := os.Getenv("ENV")
	if env == "" {
		env = "local" // Default to local
	}

	// Local development: use codex subprocess
	if env == "local" || env == "development" || env == "dev" {
		command := os.Getenv("AI_CLI_COMMAND")
		if command == "" {
			command = "codex" // Default codex for local dev
		}
		return NewCLIClient(CLIConfig{Command: command}), nil
	}

	// Production/staging: use OpenAI SDK
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY required for ENV=%s", env)
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o"
	}

	return NewOpenAIClient(Config{
		APIKey: apiKey,
		Model:  model,
	})
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
	}

	// Only set Temperature/MaxTokens for non-reasoning models
	// gpt-5* reasoning models reject these parameters
	if !isReasoningModel(c.model) {
		req.Temperature = 0.2 // Deterministic output for code generation
		req.MaxTokens = 2000  // Reasonable limit for parsing scripts
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

// isReasoningModel returns true if the model uses reasoning and doesn't support Temperature/MaxTokens.
func isReasoningModel(model string) bool {
	// gpt-5* models are reasoning models
	return strings.HasPrefix(model, "gpt-5")
}
