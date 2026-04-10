package bot

import (
	"context"
	"fmt"
)

// MockAIClient is a mock implementation of AIClient for testing
type MockAIClient struct {
	GenerateScriptFunc func(ctx context.Context, req ScriptRequest) (ScriptResponse, error)
	CallCount          int
	LastRequest        *ScriptRequest
}

func NewMockAIClient() *MockAIClient {
	return &MockAIClient{
		GenerateScriptFunc: func(ctx context.Context, req ScriptRequest) (ScriptResponse, error) {
			// Default mock behavior: return a dummy script
			return ScriptResponse{
				ScriptCode: fmt.Sprintf("// Mock script for %s %s", req.Domain, req.NodeType),
				CostUSD:    0.001,
				Model:      "mock-model",
			}, nil
		},
	}
}

func (m *MockAIClient) GenerateScript(ctx context.Context, req ScriptRequest) (ScriptResponse, error) {
	m.CallCount++
	m.LastRequest = &req
	return m.GenerateScriptFunc(ctx, req)
}

// MockScriptExecutor is a mock implementation of ScriptExecutor for testing
type MockScriptExecutor struct {
	ExecuteFunc func(ctx context.Context, script string, html string, url string) ([]RawItem, error)
	CallCount   int
	LastScript  string
	LastHTML    string
	LastURL     string
}

func NewMockScriptExecutor() *MockScriptExecutor {
	return &MockScriptExecutor{
		ExecuteFunc: func(ctx context.Context, script string, html string, url string) ([]RawItem, error) {
			// Default mock behavior: return dummy items
			return []RawItem{
				{
					Title:       "Mock Item 1",
					Description: "Mock description",
					MediaURL:    "https://example.com/image1.jpg",
					SourceURL:   url,
					MediaType:   "image",
				},
				{
					Title:       "Mock Item 2",
					Description: "Mock description 2",
					MediaURL:    "https://example.com/image2.jpg",
					SourceURL:   url,
					MediaType:   "image",
				},
			}, nil
		},
	}
}

func (m *MockScriptExecutor) Execute(ctx context.Context, script string, html string, url string) ([]RawItem, error) {
	m.CallCount++
	m.LastScript = script
	m.LastHTML = html
	m.LastURL = url
	return m.ExecuteFunc(ctx, script, html, url)
}
