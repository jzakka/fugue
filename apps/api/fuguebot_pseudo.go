package api

import "log"

/*
fugubot의 동작 플로우를 스도코드로 나타낸 것
부가 참고 자료: 정식 계약은 openspec/specs/pioneer/spec.md (Pioneer),
openspec/specs/scheduler/spec.md (URLScheduler), openspec/specs/harvester/spec.md
(Harvester)이며, 본 파일의 시그니처/순서가 정식 계약과 다르더라도 spec이
정본이다.

Deprecated: 이 파일은 의사코드이며, 실제 구현은 OpenSpec change
"scheduler-claim-api"가 정의한 scheduler.URLScheduler interface와 그 Postgres
구현체(apps/api/internal/scheduler/)로 대체된다. Pioneer 호출부는
"pioneer-scheduler-consumer" change(아카이브됨)가 정의한 PioneerConsumer
(apps/api/internal/bot/pioneer_consumer.go)로 이미 전환되었다.
*/

// Deprecated: use scheduler.URLScheduler. URLPriorityQueue는 설계 논의용
// 의사 타입이며 후속 change에서 삭제 예정.
type URLPriorityQueue struct {
}

func (pq *URLPriorityQueue) Enqueue(value ...string) {

}

// Deprecated: use scheduler.URLScheduler.Dequeue(scheduler.QueuePioneer|QueueHarvester).
// 새 interface는 QueueType enum으로 대상 테이블을 지정하며 block-on-empty /
// linearizable / host throttle 통합 의미를 가진다.
func (pq *URLPriorityQueue) Dequeue(status string) string {
	// block if pq is empty
	// pq must guarantee that linearizable.
	// status is scoring feature. not filtering criteria
	// or status would be a query
	return "https://high-value/url"
}

// Deprecated: use scheduler.URLScheduler.SetStatus(key, scheduler.Status, pinIDs).
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
