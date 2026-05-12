package bot

import "context"

// ScriptExecutor abstracts the runtime for executing parsing scripts
type ScriptExecutor interface {
	// Execute runs a script against HTML and returns extracted items
	Execute(ctx context.Context, script string, html string, url string) ([]RawItem, error)
}
