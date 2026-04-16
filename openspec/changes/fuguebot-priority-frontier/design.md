## Context

현재 Fugue bot은 `site -> graph nodes/edges -> site-specific script -> harvest` 흐름을 전제로 한다. 이 구조는 특정 사이트 내부를 반복 수확하는 데는 유리하지만, `apps/api/fuguebot_pseudo.go`가 의도하는 것처럼 URL 단위로 링크를 발견하고, 공용 우선순위 큐에서 Pioneer와 Harvester가 각자 다른 조건의 URL을 소비하는 모델과는 맞지 않는다.

현행 구조의 문제는 세 가지다.
- 탐색 전략이 BFS와 site 경계에 강하게 묶여 있어 웹 그래프 전체를 느슨하게 확장하기 어렵다.
- Harvester가 Pioneer 산출물(node type, script)에 의존해 content extraction을 시작하므로, frontier 우선순위와 수확 우선순위를 같은 모델로 다루지 못한다.
- raw HTML을 영구 저장하거나 전혀 저장하지 않는 양극단밖에 없어, 비용과 재사용 사이의 균형이 없다.

## Goals / Non-Goals

**Goals:**
- Pioneer와 Harvester를 공통 priority frontier 위의 두 worker 역할로 재정의한다.
- 사이트 중심 그래프가 아니라 URL/host/link/fetch 상태 중심 모델로 전환한다.
- Harvester가 Pioneer가 남긴 fetch snapshot을 재사용하되, snapshot이 없거나 stale하면 HTTP fallback으로 재-fetch할 수 있게 한다.
- raw HTML은 장기 TTL snapshot으로 제한하고, 검색과 수확의 영속 산출물은 searchable Pin과 링크/메타데이터로 정리한다.
- BFS/DFS를 계약에서 제거하고, 점수·쿼리 조건·재시도 시각을 가진 스케줄링 모델로 바꾼다.

**Non-Goals:**
- 이번 변경에서 전역 검색 랭킹 알고리즘(PageRank 유사 계산)까지 구현하지 않는다.
- 모든 기존 script executor 기능을 즉시 삭제하는 마이그레이션까지 강제하지 않는다.
- 대규모 분산 인프라(Kafka, separate queue service) 도입을 이번 변경의 필수 조건으로 두지 않는다.

## Decisions

### 1. Frontier는 메모리 heap이 아니라 Postgres-backed scheduler로 구현한다

`URLPriorityQueue`의 개념은 유지하되 구현은 `frontier` 테이블과 인덱스 최적화된 선별 쿼리로 둔다. 각 row는 최소한 `normalized_url`, `url`, `score`, `depth`, `host`, `next_fetch_at`, `last_fetched_at`, `fetch_error_count`, `pin_id`, `next_harvest_at`, `harvest_error_count`, `last_updated_at`를 가진다. worker는 `FOR UPDATE SKIP LOCKED`로 작업을 claim한다. `status`는 구현 내부 파생 개념일 수 있으나, 핵심 인터페이스는 “쿼리 가능한 선별 조건”이다.

이유:
- 프로세스 여러 개가 동시에 동작해도 선점 충돌을 줄일 수 있다.
- `last_fetched_at IS NULL`, `pin_id IS NULL`, `next_fetch_at <= now()`, `next_harvest_at <= now()` 같은 조건을 인덱스 친화적으로 직접 소비할 수 있다.
- 점수 기반 우선순위와 retry/backoff를 한 저장소 안에서 관리할 수 있다.

대안:
- in-memory priority queue: 단일 프로세스에서는 단순하지만, 재시작/병렬 실행/운영 관측성이 약하다.
- Redis sorted set: 가능하지만 현재 Fugue에서 트랜잭션과 관계 데이터는 Postgres가 더 자연스럽고, frontier 외 메타데이터를 따로 쪼개야 한다.

### 2. Pioneer와 Harvester는 분리하되 fetch 결과를 공유한다

Pioneer는 아직 fetch되지 않았고(`last_fetched_at IS NULL`) 현재 시각에 시도 가능한 URL을 가져와 fetch하고, 링크 추출 및 frontier 업데이트를 수행한다. 성공 fetch 시 compressed HTML snapshot을 object storage 또는 blob store에 장기 TTL과 함께 저장하고, `fetch_snapshots`에 위치와 만료시각을 기록한다. Harvester는 아직 searchable Pin으로 저장되지 않았고(`pin_id IS NULL`) fetch 결과가 존재하는 URL을 소비할 때 snapshot을 우선 조회하고, snapshot이 없거나 만료되었을 때만 HTTP fetch를 수행한다.

이유:
- Pioneer와 Harvester의 관심사를 분리하면서도 네트워크 전송 비용을 줄일 수 있다.
- Pioneer가 발견한 링크는 바로 Harvester의 입력이 될 수 있다.
- raw HTML을 DB에 영구 적재하지 않고도 단기 재사용이 가능하다.

대안:
- 단일 worker가 발견과 수확을 한 번에 처리: 구현은 간단하지만 backlog 제어와 병렬 확장이 어렵고, content extraction 실패가 link discovery를 막는다.
- 완전 분리 + fetch 비공유: 관심사 분리는 좋지만 비용이 과도하다.

### 3. raw HTML은 영구 저장이 아니라 TTL snapshot으로 제한한다

원본 HTML은 검색 인덱스의 원천이 아니라 reusable snapshot cache로 취급한다. 영속 저장 대상은 searchable Pin, 링크, fetch 메타데이터, image cache metadata다. snapshot은 압축 저장하고 장기 TTL을 둔다. 만료된 snapshot은 존재하지 않는 snapshot과 동일하게 취급한다.

이유:
- 웹 전체를 대상으로 영구 raw HTML 보관은 비용이 감당되지 않는다.
- Harvester 재시도와 장기 재사용에는 연 단위 TTL snapshot만으로 충분하다.
- Pin 자체가 검색 document이므로, 별도 문서 영속 저장보다 Pin/메타데이터가 더 직접적으로 필요하다.

대안:
- raw HTML 영구 저장: 재색인에는 유리하지만 비용이 크고 보관 정책이 무거워진다.
- raw HTML 비저장: 네트워크 재사용이 불가능하고 Pioneer/Harvester 분리의 장점이 줄어든다.

### 4. 링크 탐색은 BFS/DFS 대신 점수 기반 frontier scheduling으로 정의한다

Pioneer는 링크를 발견할 때 URL 패턴, depth penalty, DOM semantic position, query noise, retry 상태를 반영해 점수를 계산한다. Harvester도 별도의 harvest score를 사용해 수확 대기 URL을 선택한다. 구현은 queue/heap/B-tree여도 무방하되, 외부 계약은 “점수와 쿼리 가능한 조건에 따라 다음 URL을 선택한다”로 정의한다. Pioneer claim 쿼리는 기본적으로 `last_fetched_at IS NULL AND next_fetch_at <= now()`를, Harvester claim 쿼리는 `pin_id IS NULL AND last_fetched_at IS NOT NULL AND next_harvest_at <= now()`를 사용한다.

이유:
- BFS/DFS는 구현 디테일이고, 실제 요구는 “가치 높은 링크를 먼저 처리”하는 것이다.
- same-host breadth-first, deep follow, recrawl scheduling을 한 모델로 표현할 수 있다.

대안:
- 순수 BFS: 넓게 탐색은 쉬우나 가치 낮은 페이지가 과하게 섞인다.
- 순수 DFS: 깊이 편향이 심하고 링크 편향이 커진다.

### 5. Harvester의 핵심 산출물은 searchable Pin이다

Harvester는 fetch된 HTML에서 title/body_text/summary/canonical/thumbnail/media candidates를 추출하고, content page 여부를 판별해 searchable Pin을 생성하거나 보강한다. 기존 script executor는 fallback이나 특정 사이트 전용 extractor로 남길 수 있지만, 핵심 계약은 “script 실행”이 아니라 “검색 가능한 Pin 생성”이다.

이유:
- site-bound script generation은 웹 그래프 전체 탐색 모델과 잘 맞지 않는다.
- 검색엔진형 파이프라인에서 Fugue의 영속 단위는 Pin이며, Pin 테이블이 SSOT이자 검색 인덱스 역할을 한다.

### 6. 사이트 간 확장은 간단한 allow/deny 규칙과 robots 준수로 제한한다

cross-site 링크 발견은 허용하되, 간단한 키워드 기반 allow/deny 규칙과 `robots.txt`를 반드시 적용한다. 초기 버전에서는 복잡한 host politeness 모델이나 seed scope 계층화보다 단순 규칙을 우선한다.

이유:
- 웹 전체 무제한 확장은 노이즈와 운영 리스크가 너무 크다.
- 간단한 규칙과 robots 준수만으로도 초기 수집 범위를 상당히 제어할 수 있다.
- robots 조회 실패 시 전체 파이프라인을 막기보다 크롤을 진행하는 편이 초기 운영에 더 적합하다.

대안:
- 완전 자유 탐색: 확장성은 높지만 품질과 운영 안정성이 낮다.
- 정교한 host-level rate control: 필요하지만 초기 설계 필수 요소로 두면 범위가 커진다.

## Risks / Trade-offs

- [Frontier 쿼리 병목] → Pioneer claim용 partial index(`last_fetched_at IS NULL` + `next_fetch_at`, `score DESC`), Harvester claim용 partial index(`pin_id IS NULL AND last_fetched_at IS NOT NULL` + `next_harvest_at`, `score DESC`), `normalized_url` unique index로 완화하고, 필요 시 frontier만 Redis로 분리한다.
- [Snapshot TTL 만료 후 Harvester 재-fetch 증가] → 장기 TTL을 두고, 이미 Pin이 생성된 URL은 재수확 대상에서 제외한다.
- [기존 site/node/edge 모델과의 충돌] → 초기에 기존 테이블을 그대로 두고 frontier/snapshot/Pin 메타데이터 경로를 병행 도입한 뒤 읽기 경로를 전환한다.
- [script-based extractor와 document extractor 이중화] → Harvester 인터페이스를 `ExtractDocument` 중심으로 재정의하고, legacy script executor는 adapter 레이어로 격리한다.
- [cross-site crawl 확장으로 노이즈 증가] → host/domain allow/deny 키워드 규칙, path filters, robots 준수를 적용한다.

## Migration Plan

1. 새 frontier/fetch snapshot/Pin 검색 메타데이터 schema를 추가하고 Pioneer/Harvester claim 쿼리에 맞는 partial index를 만든다.
2. Pioneer를 frontier consumer/producer로 재구성하되, 기존 graph edge 저장은 병행 유지한다.
3. Harvester를 graph BFS가 아니라 frontier consumer로 전환하고, snapshot-first fetch를 붙인다.
4. searchable Pin 생성/갱신 및 이미지 캐시 메타데이터 저장을 추가한다.
5. legacy site graph/script 의존 경로를 점진적으로 축소하고, CLI/README/spec을 새 모델로 맞춘다.

## Open Questions

- 이미지 CDN 캐싱은 Pioneer 단계에서 수행할지, Harvester가 content page로 확정한 뒤에만 수행할지.
- content page 판별을 규칙 기반으로 시작할지, LLM/ML 보조 분류를 초기부터 포함할지.
- 기존 `bot_graph_nodes`/`bot_graph_edges`를 frontier 시대에도 유지할지, 아니면 `pages`/`links`로 대체할지.
