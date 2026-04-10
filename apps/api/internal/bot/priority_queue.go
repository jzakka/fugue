package bot

import "container/heap"

// QueueItem represents an item in the BFS priority queue
type QueueItem struct {
	URL      string
	URLHash  string
	Priority int
	index    int // heap index
}

// PriorityQueue implements a priority queue for BFS traversal
type PriorityQueue struct {
	items priorityQueueHeap
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		items: make(priorityQueueHeap, 0),
	}
	heap.Init(&pq.items)
	return pq
}

// Push adds an item to the queue
func (pq *PriorityQueue) Push(item *QueueItem) {
	heap.Push(&pq.items, item)
}

// Pop removes and returns the highest priority item
func (pq *PriorityQueue) Pop() *QueueItem {
	if pq.IsEmpty() {
		return nil
	}
	return heap.Pop(&pq.items).(*QueueItem)
}

// IsEmpty returns true if the queue is empty
func (pq *PriorityQueue) IsEmpty() bool {
	return len(pq.items) == 0
}

// Len returns the number of items in the queue
func (pq *PriorityQueue) Len() int {
	return len(pq.items)
}

// priorityQueueHeap implements heap.Interface
type priorityQueueHeap []*QueueItem

func (h priorityQueueHeap) Len() int { return len(h) }

// Less implements max-heap (higher priority first)
func (h priorityQueueHeap) Less(i, j int) bool {
	return h[i].Priority > h[j].Priority
}

func (h priorityQueueHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *priorityQueueHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*QueueItem)
	item.index = n
	*h = append(*h, item)
}

func (h *priorityQueueHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}
