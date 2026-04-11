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

// MockPipeline is a mock implementation of Pipeline for testing
type MockPipeline struct {
	ProcessFunc  func(ctx context.Context, items []RawItem) (pinsCreated int, deduped int, error error)
	CallCount    int
	LastItems    []RawItem
	TotalPins    int
	TotalDeduped int
}

func NewMockPipeline() *MockPipeline {
	return &MockPipeline{
		ProcessFunc: func(ctx context.Context, items []RawItem) (pinsCreated int, deduped int, error error) {
			// Default mock behavior: simulate successful processing
			return len(items), 0, nil
		},
	}
}

func (m *MockPipeline) Process(ctx context.Context, items []RawItem) (pinsCreated int, deduped int, error error) {
	m.CallCount++
	m.LastItems = items
	pinsCreated, deduped, error = m.ProcessFunc(ctx, items)
	m.TotalPins += pinsCreated
	m.TotalDeduped += deduped
	return pinsCreated, deduped, error
}
