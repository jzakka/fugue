package bot

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
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

// MockGraphRepository is a mock implementation of GraphRepository for testing
type MockGraphRepository struct {
	Nodes       map[string]db.BotGraphNode // urlHash → node
	Edges       []db.CreateEdgeParams
	nodeCounter int
}

func NewMockGraphRepository() *MockGraphRepository {
	return &MockGraphRepository{
		Nodes: make(map[string]db.BotGraphNode),
	}
}

func (m *MockGraphRepository) CreateNode(_ context.Context, arg db.CreateNodeParams) (db.BotGraphNode, error) {
	if _, exists := m.Nodes[arg.UrlHash]; exists {
		return db.BotGraphNode{}, fmt.Errorf("duplicate key value violates unique constraint")
	}
	m.nodeCounter++
	node := db.BotGraphNode{
		ID:        uuid.New(),
		SiteID:    arg.SiteID,
		Url:       arg.Url,
		UrlHash:   arg.UrlHash,
		NodeType:  arg.NodeType,
		ScriptID:  arg.ScriptID,
		SampleUrl: arg.SampleUrl,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.Nodes[arg.UrlHash] = node
	return node, nil
}

func (m *MockGraphRepository) GetNodeByHash(_ context.Context, arg db.GetNodeByHashParams) (db.BotGraphNode, error) {
	node, ok := m.Nodes[arg.UrlHash]
	if !ok {
		return db.BotGraphNode{}, sql.ErrNoRows
	}
	return node, nil
}

func (m *MockGraphRepository) GetNodeByURL(_ context.Context, arg db.GetNodeByURLParams) (db.BotGraphNode, error) {
	for _, node := range m.Nodes {
		if node.SiteID == arg.SiteID && node.Url == arg.Url {
			return node, nil
		}
	}
	return db.BotGraphNode{}, sql.ErrNoRows
}

func (m *MockGraphRepository) UpdateNodeScript(_ context.Context, _ db.UpdateNodeScriptParams) error {
	return nil
}

func (m *MockGraphRepository) ListNodesBySite(_ context.Context, _ uuid.UUID) ([]db.BotGraphNode, error) {
	var nodes []db.BotGraphNode
	for _, n := range m.Nodes {
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (m *MockGraphRepository) ListNodesByType(_ context.Context, _ db.ListNodesByTypeParams) ([]db.BotGraphNode, error) {
	return nil, nil
}

func (m *MockGraphRepository) CreateEdge(_ context.Context, arg db.CreateEdgeParams) error {
	// Check for duplicate (like ON CONFLICT DO NOTHING)
	for _, e := range m.Edges {
		if e.FromNodeID == arg.FromNodeID && e.ToNodeID == arg.ToNodeID {
			return nil
		}
	}
	m.Edges = append(m.Edges, arg)
	return nil
}

func (m *MockGraphRepository) GetEdgesByNode(_ context.Context, fromNodeID uuid.UUID) ([]db.BotGraphEdge, error) {
	var edges []db.BotGraphEdge
	for _, e := range m.Edges {
		if e.FromNodeID == fromNodeID {
			edges = append(edges, db.BotGraphEdge{
				ID:         uuid.New(),
				FromNodeID: e.FromNodeID,
				ToNodeID:   e.ToNodeID,
				CreatedAt:  time.Now(),
			})
		}
	}
	return edges, nil
}

func (m *MockGraphRepository) ListEdgesBySiteNodes(_ context.Context, _ uuid.UUID) ([]db.ListEdgesBySiteNodesRow, error) {
	var rows []db.ListEdgesBySiteNodesRow
	for _, e := range m.Edges {
		rows = append(rows, db.ListEdgesBySiteNodesRow{
			ID:         uuid.New(),
			FromNodeID: e.FromNodeID,
			ToNodeID:   e.ToNodeID,
		})
	}
	return rows, nil
}

func (m *MockGraphRepository) DeleteEdgesByIDs(_ context.Context, ids []uuid.UUID) error {
	idSet := make(map[uuid.UUID]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	// Mock: remove edges matching the given IDs
	// Since CreateEdgeParams lacks an ID field, filter by position is not possible.
	// For test purposes, just acknowledge the deletion.
	_ = idSet
	return nil
}

// MockSiteRepository is a mock implementation of SiteRepository for testing
type MockSiteRepository struct {
	Sites map[uuid.UUID]db.BotSite
}

func NewMockSiteRepository() *MockSiteRepository {
	return &MockSiteRepository{Sites: make(map[uuid.UUID]db.BotSite)}
}

func (m *MockSiteRepository) Create(_ context.Context, arg db.CreateSiteParams) (db.BotSite, error) {
	site := db.BotSite{
		ID:        uuid.New(),
		Domain:    arg.Domain,
		RootUrl:   arg.RootUrl,
		Active:    true,
		CreatedAt: time.Now(),
	}
	m.Sites[site.ID] = site
	return site, nil
}

func (m *MockSiteRepository) Get(_ context.Context, id uuid.UUID) (db.BotSite, error) {
	site, ok := m.Sites[id]
	if !ok {
		return db.BotSite{}, sql.ErrNoRows
	}
	return site, nil
}

func (m *MockSiteRepository) GetByDomain(_ context.Context, domain string) (db.BotSite, error) {
	for _, site := range m.Sites {
		if site.Domain == domain {
			return site, nil
		}
	}
	return db.BotSite{}, sql.ErrNoRows
}

func (m *MockSiteRepository) ListActive(_ context.Context) ([]db.BotSite, error) {
	var sites []db.BotSite
	for _, s := range m.Sites {
		if s.Active {
			sites = append(sites, s)
		}
	}
	return sites, nil
}

// MockScriptRepository is a mock implementation of ScriptRepository for testing
type MockScriptRepository struct{}

func NewMockScriptRepository() *MockScriptRepository {
	return &MockScriptRepository{}
}

func (m *MockScriptRepository) Create(_ context.Context, arg db.CreateScriptParams) (db.BotScript, error) {
	return db.BotScript{
		ID:       uuid.New(),
		SiteID:   arg.SiteID,
		NodeType: arg.NodeType,
	}, nil
}

func (m *MockScriptRepository) GetBySiteType(_ context.Context, _ db.GetScriptBySiteTypeParams) (db.BotScript, error) {
	return db.BotScript{}, sql.ErrNoRows
}
