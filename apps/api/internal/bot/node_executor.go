package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// NodeExecutor executes JavaScript parsing scripts using Node.js subprocess
type NodeExecutor struct {
	nodePath string // Path to node executable (default: "node")
	workDir  string // Working directory where node_modules exists
}

// NewNodeExecutor creates a new Node.js script executor
func NewNodeExecutor() *NodeExecutor {
	return &NodeExecutor{
		nodePath: "node",
		workDir:  ".", // Default to current directory
	}
}

// NewNodeExecutorWithDir creates a new Node.js script executor with custom working directory
func NewNodeExecutorWithDir(workDir string) *NodeExecutor {
	return &NodeExecutor{
		nodePath: "node",
		workDir:  workDir,
	}
}

// Execute runs a JavaScript script against HTML and returns extracted items
func (e *NodeExecutor) Execute(ctx context.Context, script string, html string, url string) ([]RawItem, error) {
	// AI-generated script is already complete - just save and run it
	// Script expects: node script.js "<html>" "<url>"

	// Create temporary script file in the workDir (where node_modules exists)
	tmpFile, err := os.CreateTemp(e.workDir, "fugue-script-*.js")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	scriptPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(scriptPath) // Cleanup: error not critical
	}()

	// Write the AI-generated script as-is (no wrapper needed)
	if _, err := tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close() // Best effort cleanup
		return nil, fmt.Errorf("write script: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Execute: node script.js "<html>" "<url>"
	cmd := exec.CommandContext(ctx, e.nodePath, scriptPath, html, url)
	cmd.Dir = e.workDir

	// Capture stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("execute script: %w (output: %s)", err, string(output))
	}

	// Parse JSON output
	var items []RawItem
	if err := json.Unmarshal(output, &items); err != nil {
		return nil, fmt.Errorf("parse output: %w (output: %s)", err, string(output))
	}

	return items, nil
}
