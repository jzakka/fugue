package bot

import "context"

// ScriptRequest contains the parameters for AI script generation
type ScriptRequest struct {
	Domain   string
	URL      string
	HTML     string
	NodeType NodeType
}

// ScriptResponse contains the generated script and metadata
type ScriptResponse struct {
	ScriptCode string
	CostUSD    float64
	Model      string
}

// AIClient abstracts the AI model interaction for script generation
type AIClient interface {
	// GenerateScript calls the AI to generate a parsing script from HTML
	// maxIterations: number of attempts to generate working script (0 = no retry, 1+ = retry with feedback)
	GenerateScript(ctx context.Context, req ScriptRequest, maxIterations int) (ScriptResponse, error)
}

// ScriptExecutor abstracts the runtime for executing parsing scripts
type ScriptExecutor interface {
	// Execute runs a script against HTML and returns extracted items
	Execute(ctx context.Context, script string, html string, url string) ([]RawItem, error)
}

// ValidationResult contains script validation outcome
type ValidationResult struct {
	Success      bool
	ItemCount    int
	ValidCount   int // Items with media_url + source_url
	ErrorMessage string
}
