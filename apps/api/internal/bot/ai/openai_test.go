package ai

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewOpenAIClient_ValidCredentials(t *testing.T) {
	cfg := Config{
		APIKey: "test-api-key",
		Model:  "gpt-4o",
	}

	client, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if client == nil {
		t.Fatal("Expected client to be initialized")
	}

	if client.model != "gpt-4o" {
		t.Errorf("Expected model gpt-4o, got: %s", client.model)
	}

	if client.timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got: %v", client.timeout)
	}
}

func TestNewOpenAIClient_MissingAPIKey(t *testing.T) {
	cfg := Config{
		Model: "gpt-5.4-codex",
	}

	client, err := NewOpenAIClient(cfg)
	if err == nil {
		t.Fatal("Expected error for missing API key, got nil")
	}

	if client != nil {
		t.Fatal("Expected client to be nil when API key is missing")
	}

	expectedMsg := "OPENAI_API_KEY is required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, err.Error())
	}
}

func TestNewOpenAIClient_DefaultModel(t *testing.T) {
	cfg := Config{
		APIKey: "test-api-key",
		// Model not specified
	}

	client, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expectedModel := "gpt-4o"
	if client.model != expectedModel {
		t.Errorf("Expected default model %s, got: %s", expectedModel, client.model)
	}
}

func TestNewOpenAIClient_CustomTimeout(t *testing.T) {
	cfg := Config{
		APIKey:  "test-api-key",
		Timeout: 60 * time.Second,
	}

	client, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if client.timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got: %v", client.timeout)
	}
}

func TestNewFromEnv_MissingAPIKey(t *testing.T) {
	// Set SDK mode but no API key
	_ = os.Setenv("AI_CLIENT_TYPE", "sdk")
	_ = os.Unsetenv("OPENAI_API_KEY")
	_ = os.Unsetenv("OPENAI_MODEL")
	defer func() { _ = os.Unsetenv("AI_CLIENT_TYPE") }()

	client, err := NewFromEnv()
	if err == nil {
		t.Fatal("Expected error for SDK mode without API key, got nil")
	}

	if client != nil {
		t.Fatal("Expected client to be nil when API key is missing")
	}

	expectedMsg := "OPENAI_API_KEY environment variable is not set (required for SDK mode)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, err.Error())
	}
}

func TestNewFromEnv_WithEnvVars(t *testing.T) {
	// Set environment variables for SDK mode
	_ = os.Setenv("AI_CLIENT_TYPE", "sdk")
	_ = os.Setenv("OPENAI_API_KEY", "test-env-key")
	_ = os.Setenv("OPENAI_MODEL", "gpt-4")
	defer func() {
		_ = os.Unsetenv("AI_CLIENT_TYPE")
		_ = os.Unsetenv("OPENAI_API_KEY")
		_ = os.Unsetenv("OPENAI_MODEL")
	}()

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Type assert to OpenAIClient
	oaiClient, ok := client.(*OpenAIClient)
	if !ok {
		t.Fatalf("Expected OpenAIClient, got: %T", client)
	}

	if oaiClient.model != "gpt-4" {
		t.Errorf("Expected model gpt-4 from env, got: %s", oaiClient.model)
	}
}

func TestNewFromEnv_DefaultModel(t *testing.T) {
	// Set only API key for SDK mode, not model
	_ = os.Setenv("AI_CLIENT_TYPE", "sdk")
	_ = os.Setenv("OPENAI_API_KEY", "test-env-key")
	defer func() {
		_ = os.Unsetenv("AI_CLIENT_TYPE")
		_ = os.Unsetenv("OPENAI_API_KEY")
	}()

	_ = os.Unsetenv("OPENAI_MODEL")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Type assert to OpenAIClient
	oaiClient, ok := client.(*OpenAIClient)
	if !ok {
		t.Fatalf("Expected OpenAIClient, got: %T", client)
	}

	expectedModel := "gpt-4o"
	if oaiClient.model != expectedModel {
		t.Errorf("Expected default model %s, got: %s", expectedModel, oaiClient.model)
	}
}

func TestCall_ContextCancellation(t *testing.T) {
	cfg := Config{
		APIKey: "test-api-key",
		Model:  "gpt-5.4-codex",
	}

	client, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.Call(ctx, "test prompt")
	if err == nil {
		t.Fatal("Expected error for cancelled context, got nil")
	}
}

func TestCall_Timeout(t *testing.T) {
	cfg := Config{
		APIKey:  "test-api-key",
		Model:   "gpt-5.4-codex",
		Timeout: 1 * time.Nanosecond, // Very short timeout to trigger
	}

	client, err := NewOpenAIClient(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	ctx := context.Background()
	_, err = client.Call(ctx, "test prompt")
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Note: Actual API call will fail, but we're testing timeout behavior
}
