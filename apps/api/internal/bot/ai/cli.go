package ai

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CLIClient implements the Client interface by calling a local CLI command.
// By default uses "codex" CLI for local development.
type CLIClient struct {
	command string   // CLI command to execute (default: "codex")
	args    []string // Additional arguments to pass
}

// CLIConfig holds configuration for the CLI client.
type CLIConfig struct {
	// Command is the CLI executable name (default: "codex")
	Command string

	// Args are additional arguments to pass to the command
	Args []string
}

// NewCLIClient creates a new CLI-based AI client.
func NewCLIClient(cfg CLIConfig) *CLIClient {
	if cfg.Command == "" {
		cfg.Command = "codex" // Default to codex, not chatgpt
	}

	return &CLIClient{
		command: cfg.Command,
		args:    cfg.Args,
	}
}

// buildArgs returns the final argument list for execution.
// For codex, it prepends "exec -" for non-interactive mode.
func (c *CLIClient) buildArgs() []string {
	if c.command == "codex" {
		return append([]string{"exec", "-"}, c.args...)
	}
	return c.args
}

// Call sends a prompt to the CLI and returns the response.
func (c *CLIClient) Call(ctx context.Context, prompt string) (string, error) {
	args := c.buildArgs()
	cmd := exec.CommandContext(ctx, c.command, args...)
	cmd.Stdin = strings.NewReader(prompt)

	// Separate stdout and stderr to avoid pollution
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("CLI command failed: %w (stderr: %s)", err, stderr.String())
	}

	// Return only stdout (script), ignore stderr (warnings/progress)
	response := strings.TrimSpace(stdout.String())
	if response == "" {
		return "", fmt.Errorf("CLI returned empty response")
	}

	return response, nil
}
