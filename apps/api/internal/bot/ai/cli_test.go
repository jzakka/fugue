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

	if client.command != "codex" {
		t.Errorf("Expected default command 'codex', got: %s", client.command)
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

func TestCLIClient_BuildArgs_CodexInjectsExec(t *testing.T) {
	client := NewCLIClient(CLIConfig{})
	args := client.buildArgs()

	if len(args) < 2 || args[0] != "exec" || args[1] != "-" {
		t.Errorf("Expected codex to inject [exec, -], got: %v", args)
	}
}

func TestCLIClient_BuildArgs_CodexWithExistingArgs(t *testing.T) {
	client := NewCLIClient(CLIConfig{
		Command: "codex",
		Args:    []string{"--model", "gpt-4"},
	})
	args := client.buildArgs()

	expected := []string{"exec", "-", "--model", "gpt-4"}
	if len(args) != len(expected) {
		t.Fatalf("Expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, v := range expected {
		if args[i] != v {
			t.Errorf("args[%d] = %q, want %q", i, args[i], v)
		}
	}
}

func TestCLIClient_BuildArgs_NonCodexNoInjection(t *testing.T) {
	client := NewCLIClient(CLIConfig{
		Command: "custom-ai",
		Args:    []string{"--flag"},
	})
	args := client.buildArgs()

	if len(args) != 1 || args[0] != "--flag" {
		t.Errorf("Expected non-codex args unchanged [--flag], got: %v", args)
	}
}

func TestCLIClient_BuildArgs_OriginalArgsUnmodified(t *testing.T) {
	client := NewCLIClient(CLIConfig{
		Command: "codex",
		Args:    []string{"--model", "gpt-4"},
	})

	// Call buildArgs twice to ensure c.args is not mutated
	_ = client.buildArgs()
	args2 := client.buildArgs()

	expected := []string{"exec", "-", "--model", "gpt-4"}
	if len(args2) != len(expected) {
		t.Fatalf("Second call: expected %d args, got %d: %v", len(expected), len(args2), args2)
	}
	for i, v := range expected {
		if args2[i] != v {
			t.Errorf("Second call: args[%d] = %q, want %q", i, args2[i], v)
		}
	}
}

func TestNewFromEnv_CLIMode(t *testing.T) {
	// Clear all AI env vars
	_ = os.Unsetenv("ENV")
	_ = os.Unsetenv("OPENAI_API_KEY")
	_ = os.Unsetenv("OPENAI_MODEL")
	_ = os.Unsetenv("AI_CLI_COMMAND")

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
	_ = os.Setenv("ENV", "local")
	defer func() { _ = os.Unsetenv("ENV") }()

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if _, ok := client.(*CLIClient); !ok {
		t.Errorf("Expected CLIClient, got: %T", client)
	}
}

func TestNewFromEnv_DevMode(t *testing.T) {
	_ = os.Setenv("ENV", "dev")
	defer func() { _ = os.Unsetenv("ENV") }()

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if _, ok := client.(*CLIClient); !ok {
		t.Errorf("Expected CLIClient in dev mode, got: %T", client)
	}
}

func TestNewFromEnv_ProductionMode_MissingKey(t *testing.T) {
	_ = os.Setenv("ENV", "production")
	_ = os.Unsetenv("OPENAI_API_KEY")
	defer func() { _ = os.Unsetenv("ENV") }()

	client, err := NewFromEnv()
	if err == nil {
		t.Fatal("Expected error for production mode without API key, got nil")
	}

	if client != nil {
		t.Fatal("Expected client to be nil when API key is missing")
	}
}

func TestNewFromEnv_ProductionMode_WithKey(t *testing.T) {
	_ = os.Setenv("ENV", "production")
	_ = os.Setenv("OPENAI_API_KEY", "test-key")
	defer func() {
		_ = os.Unsetenv("ENV")
		_ = os.Unsetenv("OPENAI_API_KEY")
	}()

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("Expected OpenAIClient in production mode, got: %T", client)
	}
}

func TestNewFromEnv_CustomCLICommand(t *testing.T) {
	_ = os.Setenv("ENV", "local")
	_ = os.Setenv("AI_CLI_COMMAND", "custom-codex")
	defer func() {
		_ = os.Unsetenv("ENV")
		_ = os.Unsetenv("AI_CLI_COMMAND")
	}()

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	cliClient, ok := client.(*CLIClient)
	if !ok {
		t.Fatalf("Expected CLIClient, got: %T", client)
	}

	if cliClient.command != "custom-codex" {
		t.Errorf("Expected custom command 'custom-codex', got: %s", cliClient.command)
	}
}
