package visualize

import (
	"encoding/json"
	"fmt"
)

// SerializeGraphData converts GraphData to JSON
func SerializeGraphData(data *GraphData) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("graph data is nil")
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal graph data: %w", err)
	}

	return jsonBytes, nil
}
