## 1. 분기 순서 수정

- [x] 1.1 `apps/api/internal/bot/harvester_consumer.go`의 `processOne` 안에서 `doc, fellBack, extractErr := ...` 직후 `if fellBack { h.stats.adapterFallback.Add(1) }` 분기를 `extractErr` 검사보다 앞에 위치시킨다. 기존 위치(`extractErr == nil` 분기 뒤)는 제거한다. design.md Decision 1.
- [x] 1.2 변경 라인 근처에 "spec: harvester `Harvester 노드 단위 통계 정의` — 본문 SHALL 보강(어댑터 실패가 발생하면 generic 성공/실패와 무관하게 AdapterFallback이 1 증가)과 Scenario `어댑터 실패 후 generic 실패`를 enforce" 한 줄 주석을 남겨 회귀 시 의도가 보이도록 한다.

## 2. 회귀 테스트

- [x] 2.1 `apps/api/internal/bot/harvester_consumer_test.go`(또는 신규 테스트 파일)에 어댑터 실패+generic 실패 경로를 시나리오로 만드는 `func Test...` 테스트를 추가한다. 테스트는 (i) 기존 `failingAdapter` 헬퍼로 어댑터 실패를 만들고, (ii) `Extract`가 항상 에러를 반환하는 `genericExtractorIface` stub을 `withExtractor`로 주입하여 generic 실패를 만든 뒤, (iii) `processOne` 처리 후 `consumer.NodeStats()`의 `Failed`가 1, `AdapterFallback`이 1임을 확인한다. design.md Decision 3.
- [x] 2.2 기존 "어댑터 실패+generic 성공" 경로 회귀 방지: 그 케이스에서도 `AdapterFallback==1`이 유지됨을 새 또는 기존 테스트로 보장한다(기존 `TestHarvesterConsumer_NodeStats_AdapterFallback`가 이를 커버하면 별도 추가 불필요).

## 3. 검증

- [x] 3.1 `cd apps/api && go build ./...` 통과.
- [x] 3.2 `cd apps/api && go test ./internal/bot/...` 통과.
- [x] 3.3 `openspec validate fix-harvester-adapter-fallback-counter --strict` 통과.
