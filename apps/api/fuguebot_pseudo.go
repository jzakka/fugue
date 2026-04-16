package api

import "log"

/*
fugubot의 동작 플로우를 스도코드로 나타낸 것
참고만 하시오
*/

type URLPriorityQueue struct {
}

func (pq *URLPriorityQueue) Enqueue(value ...string) {

}

func (pq *URLPriorityQueue) Dequeue(status string) string {
	// block if pq is empty
	// pq must guarantee that linearizable.
	// status is scoring feature. not filtering criteria
	// or status would be a query
	return "https://high-value/url"
}

func (pq *URLPriorityQueue) SetStatus(key string, s string) {

}

func Key(content []byte) string {
	return ""
}

type Pioneer struct {
	maxDepth int // or infinity
	pq       *URLPriorityQueue
	f        Fetcher
}

func (p Pioneer) Run() {
	current := p.pq.Dequeue("not-visited")

	for i := 0; i < p.maxDepth; i++ {
		var (
			content []byte
			err     error
		)
		for retry := 0; retry < 3; retry++ {
			content, err = p.f.Fetch(current)
			if err == nil {
				break
			}
		}
		if err != nil {
			log.Printf("..로그 ")
			current = p.pq.Dequeue("not-visited")
			continue
		}

		links := p.ParseLinks(content)
		links = p.FilterLinks(links)
		p.pq.Enqueue(links...)

		if err := p.SaveRawContent(content); err == nil {
			p.pq.SetStatus(Key(content), "pending")
		}
		current = p.pq.Dequeue("not-visited")
	}
}

func (p Pioneer) ParseLinks([]byte) []string {
	return []string{"https://high-value/1", "https://high-value/2", "https://high-value/3"}
}

func (p Pioneer) FilterLinks(links []string) []string {
	return links
}

func (p Pioneer) SaveRawContent(content []byte) error {
	return nil
}

type Fetcher interface {
	Fetch(url string) ([]byte, error)
}

type CompositeFetcher struct {
	o ObjectStorageFetcher
	h HTTPFetcher
}

func (f CompositeFetcher) Fetch(url string) ([]byte, error) {
	res, err := f.o.Fetch(url)
	if err != nil {
		return f.h.Fetch(url)
	}
	return res, nil
}

type ObjectStorageFetcher struct{}

func (f ObjectStorageFetcher) Fetch(url string) ([]byte, error) {
	return nil, nil
}

type HTTPFetcher struct{}

func (f HTTPFetcher) Fetch(url string) ([]byte, error) {
	return nil, nil
}

type Harvester struct {
	maxDepth int // or infinity
	pq       *URLPriorityQueue
	f        Fetcher
}

func (h Harvester) Run() {
	current := h.pq.Dequeue("not-parsed")

	for i := 0; i < h.maxDepth; i++ {
		var (
			content []byte
			err     error
		)
		for retry := 0; retry < 3; retry++ {
			content, err = h.f.Fetch(current)
			if err == nil {
				break
			}
		}
		if err != nil {
			log.Printf("...로그")
			current = h.pq.Dequeue("not-parsed")
			continue
		}

		document := h.ParseDocument(content)
		if err := h.Index(document); err == nil {
			log.Printf("...로그")
		}
		current = h.pq.Dequeue("not-parsed")
	}
}

func (h Harvester) ParseDocument(content []byte) interface{} {
	return nil
}

func (h Harvester) Index(document interface{}) error {
	return nil
}
