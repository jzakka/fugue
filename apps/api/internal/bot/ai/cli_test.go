package ai

import (
	"context"
	"os"
	"testing"
)

func TestNewCLIClient(t *testing.T) {
	client := NewCLIClient(CLIConfig{})

	if client == nil {
		t.Fatal("Expected client to be initialized")
	}

	if client.command != "chatgpt" {
		t.Errorf("Expected default command 'chatgpt', got: %s", client.command)
	}
}

func TestNewCLIClient_CustomCommand(t *testing.T) {
	cfg := CLIConfig{
		Command: "custom-ai",
		Args:    []string{"--model", "gpt-4"},
	}

	client := NewCLIClient(cfg)

	if client.command != "custom-ai" {
		t.Errorf("Expected command 'custom-ai', got: %s", client.command)
	}

	if len(client.args) != 2 {
		t.Errorf("Expected 2 args, got: %d", len(client.args))
	}
}

func TestCLIClient_Call_CommandNotFound(t *testing.T) {
	// Use a command that definitely doesn't exist
	client := NewCLIClient(CLIConfig{
		Command: "nonexistent-ai-command-12345",
	})

	ctx := context.Background()
	_, err := client.Call(ctx, "test prompt")

	if err == nil {
		t.Fatal("Expected error for nonexistent command, got nil")
	}
}

func TestCLIClient_Call_ContextCancellation(t *testing.T) {
	client := NewCLIClient(CLIConfig{
		Command: "sleep",
		Args:    []string{"10"}, // Sleep for 10 seconds
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Call(ctx, "ignored")
	if err == nil {
		t.Fatal("Expected error for cancelled context, got nil")
	}
}

func TestNewFromEnv_CLIMode(t *testing.T) {
	// Clear all AI env vars
	_ = os.Unsetenv("AI_CLIENT_TYPE")
	_ = os.Unsetenv("OPENAI_API_KEY")
	_ = os.Unsetenv("OPENAI_MODEL")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error for CLI mode, got: %v", err)
	}

	if client == nil {
		t.Fatal("Expected client to be initialized")
	}

	// Verify it's a CLI client by type assertion
	if _, ok := client.(*CLIClient); !ok {
		t.Errorf("Expected CLIClient, got: %T", client)
	}
}

func TestNewFromEnv_CLIModeExplicit(t *testing.T) {
	_ = os.Setenv("AI_CLIENT_TYPE", "cli")
	defer func() { _ = os.Unsetenv("AI_CLIENT_TYPE") }()

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if _, ok := client.(*CLIClient); !ok {
		t.Errorf("Expected CLIClient, got: %T", client)
	}
}

func TestNewFromEnv_SDKMode_MissingKey(t *testing.T) {
	_ = os.Setenv("AI_CLIENT_TYPE", "sdk")
	_ = os.Unsetenv("OPENAI_API_KEY")
	defer func() { _ = os.Unsetenv("AI_CLIENT_TYPE") }()

	client, err := NewFromEnv()
	if err == nil {
		t.Fatal("Expected error for SDK mode without API key, got nil")
	}

	if client != nil {
		t.Fatal("Expected client to be nil when API key is missing")
	}
}

func TestNewFromEnv_SDKMode_WithKey(t *testing.T) {
	_ = os.Setenv("AI_CLIENT_TYPE", "sdk")
	_ = os.Setenv("OPENAI_API_KEY", "test-key")
	defer func() {
		_ = os.Unsetenv("AI_CLIENT_TYPE")
		_ = os.Unsetenv("OPENAI_API_KEY")
	}()

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("Expected OpenAIClient in SDK mode, got: %T", client)
	}
}

func TestNewFromEnv_InvalidType(t *testing.T) {
	_ = os.Setenv("AI_CLIENT_TYPE", "invalid-type")
	defer func() { _ = os.Unsetenv("AI_CLIENT_TYPE") }()

	client, err := NewFromEnv()
	if err == nil {
		t.Fatal("Expected error for invalid client type, got nil")
	}

	if client != nil {
		t.Fatal("Expected client to be nil")
	}
}
