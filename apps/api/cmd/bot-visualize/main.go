package main

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	_ "embed"

	_ "github.com/lib/pq"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/cmd/visualize"
	"github.com/chungsanghwa/fugue/apps/api/internal/db"
)

//go:embed template.html
var htmlTemplate string

func main() {
	format := flag.String("format", "html", "Output format: html, png, svg")
	output := flag.String("output", "graph.html", "Output file path")
	filterSite := flag.String("filter-site", "", "Filter by site domain")
	flag.Parse()

	if err := run(*format, *output, *filterSite); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}

func run(format, output, filterSite string) error {
	ctx := context.Background()

	// Get DATABASE_URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// Connect to DB
	dbConn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		_ = dbConn.Close()
	}()

	if err := dbConn.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	queries := db.New(dbConn)
	repo := visualize.NewGraphRepository(queries)

	// Fetch graph data
	graphData, err := repo.FetchGraphData(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch graph data: %w", err)
	}

	// Handle empty graph
	if len(graphData.Nodes) == 0 {
		fmt.Println("No nodes found in database. Run Pioneer first to crawl some sites.")
		return nil
	}

	// Apply site filter if specified
	if filterSite != "" {
		graphData = filterBySite(graphData, filterSite)
		if len(graphData.Nodes) == 0 {
			fmt.Printf("No nodes found for site: %s\n", filterSite)
			return nil
		}
	}

	// Generate output based on format
	switch format {
	case "html":
		return generateHTML(graphData, output)
	case "png", "svg":
		return generateGraphviz(graphData, output, format)
	default:
		return fmt.Errorf("unsupported format: %s (supported: html, png, svg)", format)
	}
}

func generateHTML(data *visualize.GraphData, outputPath string) error {
	// Serialize graph data to JSON
	jsonBytes, err := visualize.SerializeGraphData(data)
	if err != nil {
		return fmt.Errorf("failed to serialize graph data: %w", err)
	}

	// Parse template
	tmpl, err := template.New("graph").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	templateData := struct {
		GraphDataJSON template.JS
	}{
		GraphDataJSON: template.JS(jsonBytes),
	}

	if err := tmpl.Execute(&buf, templateData); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	absPath, _ := filepath.Abs(outputPath)
	fmt.Printf("✓ Graph visualization generated: %s\n", absPath)
	fmt.Printf("  Open in browser: file://%s\n", absPath)
	fmt.Printf("  %d sites, %d nodes, %d edges\n",
		data.Metadata.TotalSites, data.Metadata.TotalNodes, data.Metadata.TotalEdges)
	return nil
}

func generateGraphviz(data *visualize.GraphData, outputPath, format string) error {
	// Check if Graphviz is installed
	if !visualize.IsGraphvizInstalled() {
		return fmt.Errorf("graphviz is not installed. Install with: brew install graphviz")
	}

	// Generate DOT format
	dotContent, err := visualize.GenerateDOT(data)
	if err != nil {
		return fmt.Errorf("failed to generate DOT: %w", err)
	}

	// Export via Graphviz
	if err := visualize.ExportGraphviz(dotContent, outputPath, format); err != nil {
		return fmt.Errorf("failed to export %s: %w", format, err)
	}

	absPath, _ := filepath.Abs(outputPath)
	fmt.Printf("✓ Graph exported as %s: %s\n", format, absPath)
	return nil
}

func filterBySite(data *visualize.GraphData, domain string) *visualize.GraphData {
	// Find matching site
	var targetSiteID *string
	filteredSites := []visualize.Site{}
	for _, site := range data.Sites {
		if site.Domain == domain {
			id := site.ID.String()
			targetSiteID = &id
			filteredSites = append(filteredSites, site)
			break
		}
	}

	if targetSiteID == nil {
		return &visualize.GraphData{
			Sites:    []visualize.Site{},
			Nodes:    []visualize.Node{},
			Edges:    []visualize.Edge{},
			Metadata: data.Metadata,
		}
	}

	// Filter nodes
	filteredNodes := []visualize.Node{}
	nodeIDs := make(map[string]bool)
	for _, node := range data.Nodes {
		if node.SiteID.String() == *targetSiteID {
			filteredNodes = append(filteredNodes, node)
			nodeIDs[node.ID.String()] = true
		}
	}

	// Filter edges
	filteredEdges := []visualize.Edge{}
	for _, edge := range data.Edges {
		if nodeIDs[edge.FromNodeID.String()] && nodeIDs[edge.ToNodeID.String()] {
			filteredEdges = append(filteredEdges, edge)
		}
	}

	return &visualize.GraphData{
		Sites: filteredSites,
		Nodes: filteredNodes,
		Edges: filteredEdges,
		Metadata: visualize.Metadata{
			GeneratedAt: data.Metadata.GeneratedAt,
			TotalSites:  len(filteredSites),
			TotalNodes:  len(filteredNodes),
			TotalEdges:  len(filteredEdges),
		},
	}
}
