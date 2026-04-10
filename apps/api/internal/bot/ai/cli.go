package ai

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CLIClient implements the Client interface by calling a local chatgpt CLI command.
// This is useful when you have ChatGPT Plus subscription but no API key.
type CLIClient struct {
	command string   // CLI command to execute (default: "chatgpt")
	args    []string // Additional arguments to pass
}

// CLIConfig holds configuration for the CLI client.
type CLIConfig struct {
	// Command is the CLI executable name (default: "chatgpt")
	Command string

	// Args are additional arguments to pass to the command
	Args []string
}

// NewCLIClient creates a new CLI-based AI client.
func NewCLIClient(cfg CLIConfig) *CLIClient {
	if cfg.Command == "" {
		cfg.Command = "chatgpt"
	}

	return &CLIClient{
		command: cfg.Command,
		args:    cfg.Args,
	}
}

// Call sends a prompt to the chatgpt CLI and returns the response.
func (c *CLIClient) Call(ctx context.Context, prompt string) (string, error) {
	// Build command: chatgpt [args...] <prompt>
	args := append(c.args, prompt)
	cmd := exec.CommandContext(ctx, c.command, args...)

	// Capture stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("CLI command failed: %w (output: %s)", err, string(output))
	}

	// Trim whitespace from response
	response := strings.TrimSpace(string(output))
	if response == "" {
		return "", fmt.Errorf("CLI returned empty response")
	}

	return response, nil
}
