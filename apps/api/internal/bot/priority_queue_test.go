package bot

import (
	"testing"
)

// Test priority queue behavior
func TestPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue()

	// Add items with different priorities
	pq.Push(&QueueItem{URL: "low", Priority: 10})
	pq.Push(&QueueItem{URL: "high", Priority: 100})
	pq.Push(&QueueItem{URL: "medium", Priority: 50})

	// Should pop in priority order (highest first)
	if item := pq.Pop(); item.URL != "high" {
		t.Errorf("Expected 'high' first, got %s", item.URL)
	}
	if item := pq.Pop(); item.URL != "medium" {
		t.Errorf("Expected 'medium' second, got %s", item.URL)
	}
	if item := pq.Pop(); item.URL != "low" {
		t.Errorf("Expected 'low' third, got %s", item.URL)
	}

	if !pq.IsEmpty() {
		t.Error("Queue should be empty")
	}
}

func TestPriorityQueueEmpty(t *testing.T) {
	pq := NewPriorityQueue()

	if !pq.IsEmpty() {
		t.Error("New queue should be empty")
	}

	if pq.Len() != 0 {
		t.Errorf("Empty queue length should be 0, got %d", pq.Len())
	}

	if item := pq.Pop(); item != nil {
		t.Error("Pop from empty queue should return nil")
	}
}

func TestPriorityQueueSamePriority(t *testing.T) {
	pq := NewPriorityQueue()

	// Add items with same priority - should maintain insertion order
	pq.Push(&QueueItem{URL: "first", Priority: 50})
	pq.Push(&QueueItem{URL: "second", Priority: 50})
	pq.Push(&QueueItem{URL: "third", Priority: 50})

	// Just verify we get all items back
	count := 0
	for !pq.IsEmpty() {
		pq.Pop()
		count++
	}

	if count != 3 {
		t.Errorf("Expected to pop 3 items, got %d", count)
	}
}
