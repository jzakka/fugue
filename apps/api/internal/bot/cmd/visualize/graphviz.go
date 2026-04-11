package visualize

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsGraphvizInstalled checks if the dot command is available
func IsGraphvizInstalled() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}

// GenerateDOT creates Graphviz DOT format from graph data
func GenerateDOT(data *GraphData) (string, error) {
	var sb strings.Builder

	sb.WriteString("digraph FugueBot {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=circle, style=filled, fontsize=10];\n")
	sb.WriteString("  edge [color=\"#404040\", arrowsize=0.7];\n\n")

	// Group by site
	siteMap := make(map[string][]Node)
	for _, node := range data.Nodes {
		siteID := node.SiteID.String()
		siteMap[siteID] = append(siteMap[siteID], node)
	}

	// Create subgraphs for each site
	for i, site := range data.Sites {
		fmt.Fprintf(&sb, "  subgraph cluster_%d {\n", i)
		fmt.Fprintf(&sb, "    label=\"%s\";\n", site.Domain)
		sb.WriteString("    style=filled;\n")
		sb.WriteString("    color=\"#333333\";\n")
		sb.WriteString("    fillcolor=\"#1a1a1a\";\n")
		sb.WriteString("    fontcolor=\"#cccccc\";\n\n")

		// Add nodes for this site
		for _, node := range siteMap[site.ID.String()] {
			color := "#6b7280" // gray - uncovered
			if node.HasScript {
				color = "#10b981" // green - covered
			}

			label := node.NodeType
			if label == "" {
				label = "unknown"
			}

			fmt.Fprintf(&sb, "    \"%s\" [label=\"%s\", fillcolor=\"%s\"];\n",
				node.ID.String(), label, color)
		}

		sb.WriteString("  }\n\n")
	}

	// Add edges
	for _, edge := range data.Edges {
		fmt.Fprintf(&sb, "  \"%s\" -> \"%s\";\n",
			edge.FromNodeID.String(), edge.ToNodeID.String())
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}

// ExportGraphviz exports DOT content to PNG or SVG using Graphviz
func ExportGraphviz(dotContent, outputPath, format string) error {
	// Write DOT to temp file
	tmpFile, err := os.CreateTemp("", "fugue-graph-*.dot")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.WriteString(dotContent); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write DOT content: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Run Graphviz
	if format != "svg" {
		format = "png"
	}

	cmd := exec.Command("dot", fmt.Sprintf("-T%s", format), tmpFile.Name(), "-o", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("graphviz failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
