package bot

import (
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// BFSQueue holds nodes for level-based BFS traversal
// Nodes are organized by depth levels to ensure BFS order
type BFSQueue struct {
	levels [][]db.BotGraphNode // Each level contains nodes at that depth
}

// NewBFSQueue creates a new BFS queue
func NewBFSQueue() *BFSQueue {
	return &BFSQueue{
		levels: make([][]db.BotGraphNode, 0),
	}
}

// AddLevel adds nodes to a new level (depth)
func (q *BFSQueue) AddLevel(nodes []db.BotGraphNode) {
	if len(nodes) > 0 {
		q.levels = append(q.levels, nodes)
	}
}

// PopLevel removes and returns all nodes at the current depth
// Returns empty slice if queue is empty
func (q *BFSQueue) PopLevel() []db.BotGraphNode {
	if q.IsEmpty() {
		return []db.BotGraphNode{}
	}
	level := q.levels[0]
	q.levels = q.levels[1:]
	return level
}

// IsEmpty returns true if there are no more levels
func (q *BFSQueue) IsEmpty() bool {
	return len(q.levels) == 0
}

// Len returns the total number of nodes across all levels
func (q *BFSQueue) Len() int {
	total := 0
	for _, level := range q.levels {
		total += len(level)
	}
	return total
}
