# Pioneer Specification

## Purpose

Fugue의 Pioneer crawler capability. Pioneer는 `URLScheduler`의 얇은 consumer로 정의되며, `pioneer_frontier`에서 `Dequeue`로 다음 URL을 받아 fetch · snapshot 저장 · 링크 추출 · 필터 통과 링크의 `Enqueue` · 원본 URL+snapshot_key의 `EnqueueHarvester` · `SetStatus` 보고를 반복한다. 인메모리 큐/visited 맵/사이트 세션 상태를 보유하지 않으며, 다중 워커 동시성·dedup·ordering은 모두 scheduler가 보장한다.
## Requirements
### Requirement: Pioneer는 URLScheduler의 consumer이다
Pioneer는 URL frontier를 `URLScheduler` 인터페이스를 통해서만 읽어야 하며(SHALL), 자체 URL 큐/스택/BFS 자료구조를 보유해서는 안 된다(SHALL NOT). 크롤할 다음 URL은 `scheduler.Dequeue(scheduler.QueuePioneer)` 호출로만 획득한다.

#### Scenario: Dequeue(QueuePioneer)로만 다음 URL을 획득한다
- **WHEN** Pioneer가 다음으로 크롤할 URL을 결정할 때
- **THEN** `scheduler.Dequeue(scheduler.QueuePioneer)`를 호출하여 URL을 얻으며, 내부 큐/스택/리스트에서 꺼내지 않는다

#### Scenario: Dequeue 시그니처는 QueueType 단일 인자이다
- **WHEN** Pioneer가 `Dequeue`를 호출할 때
- **THEN** 인자는 `scheduler.QueuePioneer` 하나이며, `queryCondition` 같은 추가 문자열 파라미터를 전달하지 않는다 (partial index 조건은 scheduler 구현체에 내재되어 있다)

#### Scenario: 자체 URL 큐를 보유하지 않는다
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** Pioneer 내부에 URL을 누적하는 큐/스택/슬라이스/채널 상태가 존재하지 않는다 (scheduler가 유일한 큐이다)

#### Scenario: frontier가 비어 있으면 Dequeue가 블록된다
- **WHEN** `pioneer_frontier`에 claim 가능한 URL이 없는 상태에서 Pioneer가 `scheduler.Dequeue(scheduler.QueuePioneer)`를 호출할 때
- **THEN** Pioneer는 scheduler가 URL을 반환할 때까지 대기한다 (scheduler의 block-on-empty 대기 정책을 그대로 따르며 Pioneer 루프 측에서 별도 `time.Sleep`이나 스핀을 구현하지 않는다. 내부 폴링 간격은 scheduler spec 책임)

---

### Requirement: Pioneer 메인 루프는 Dequeue → fetch → snapshot → parse → filter → Enqueue(pioneer) + EnqueueHarvester → SetStatus 반복이다
Pioneer의 메인 루프는 `scheduler.Dequeue(QueuePioneer)`, URL fetch, snapshot 저장, 링크 추출, `FilterChain.Apply`, `scheduler.Enqueue(QueuePioneer, filteredURLs)`, `scheduler.EnqueueHarvester(url, snapshotKey)`, `scheduler.SetStatus(url, "fetched", nil)`의 반복으로 구성되어야 한다(SHALL). 추가 단계를 이 루프의 책임으로 포함하지 않아야 한다(SHALL NOT).

#### Scenario: 정상 경로의 루프 순서
- **WHEN** Pioneer가 한 번의 성공 반복을 수행할 때
- **THEN** 순서대로 `scheduler.Dequeue(QueuePioneer)` → URL fetch → snapshot 저장(`snapshot_key` 획득) → 링크 추출 → `FilterChain.Apply` → `scheduler.Enqueue(QueuePioneer, filteredURLs)` → `scheduler.EnqueueHarvester(url, snapshotKey)` → `scheduler.SetStatus(url, "fetched", nil)` 이 실행된다.

#### Scenario: FilterChain은 Enqueue 직전에 Pioneer consumer가 호출한다
- **WHEN** Pioneer가 링크 목록을 `pioneer_frontier`에 넣기 직전일 때
- **THEN** Pioneer consumer가 `filterChain.Apply(links)`를 호출하여 필터를 통과한 링크만 `scheduler.Enqueue(QueuePioneer, ...)`로 투입한다 (필터 구성 자체는 `pioneer-link-filter-policy`가 정의하며, Pioneer는 호출 타이밍만 책임진다).

#### Scenario: 추출한 링크를 같은 pioneer_frontier로 다시 Enqueue한다
- **WHEN** 한 URL의 fetch 결과에서 n개 링크를 추출하여 필터를 통과시켰을 때
- **THEN** Pioneer는 `scheduler.Enqueue(scheduler.QueuePioneer, filteredURLs)`를 호출하여 동일 scheduler의 `pioneer_frontier`에 다시 투입한다 (별도 큐/채널/파일로 내보내지 않는다).

#### Scenario: 루프에 별도 sleep/backoff를 두지 않는다
- **WHEN** Pioneer consumer 루프 코드를 정적 분석할 때
- **THEN** 빈 큐 폴링이나 실패 재시도를 위한 `time.Sleep`/`time.After` 호출이 루프 본문에 존재하지 않는다 (폴링 책임은 scheduler 내부에 있다).

#### Scenario: 루프는 상기 단계를 반복한다
- **WHEN** Pioneer 프로세스가 정상 구동 중일 때
- **THEN** 메인 루프는 상기 단계를 반복하며, 워커 종료 조건(work budget 소진)은 본 capability의 "Pioneer 워커는 성공 Dequeue 100회 후 종료한다" requirement에 정의되어 있다 (이 루프 스펙 내에서는 단계 순서만 규범화한다).

### Requirement: Pioneer는 성공 시 SetStatus("fetched", nil)로만 보고한다
Pioneer는 fetch 성공 시 `scheduler.SetStatus(url, "fetched", nil)`를 호출해야 한다(SHALL). `pinIDs` 인자는 항상 `nil`이어야 한다(SHALL). Pioneer가 frontier 테이블 컬럼을 직접 UPDATE해서는 안 된다(SHALL NOT).

#### Scenario: fetch 성공 시 SetStatus("fetched", nil) 호출
- **WHEN** Pioneer가 URL fetch에 성공하고 snapshot 저장, 링크 추출, Enqueue(pioneer), EnqueueHarvester를 모두 완료했을 때
- **THEN** `scheduler.SetStatus(url, "fetched", nil)`를 호출한다 (scheduler 구현체가 `next_fetch_at = now() + 365 days`, `fetch_error_count = 0`으로 갱신한다)

#### Scenario: pinIDs 인자는 nil이다
- **WHEN** Pioneer가 `SetStatus`를 호출할 때
- **THEN** `pinIDs` 인자는 항상 `nil`이다 (Pin 생성은 Harvester의 책임이며 Pioneer는 pin ID를 알지 못한다)

#### Scenario: frontier 컬럼 직접 UPDATE 금지
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** Pioneer가 `pioneer_frontier` / `harvester_frontier` 테이블의 `last_fetched_at` / `fetch_error_count` / `next_fetch_at` / `snapshot_key` / `harvested_at` 등 컬럼을 직접 UPDATE/INSERT하는 코드가 존재하지 않는다 (모든 상태 변경은 `URLScheduler` API를 경유한다)

---

### Requirement: Pioneer는 실패 시 SetStatus + RecordFetchError 둘 다 호출한다
Pioneer는 URL fetch 또는 snapshot 저장 실패 시 `scheduler.SetStatus(url, "fetch_failed", nil)`와 `scheduler.RecordFetchError(url, errorKind)`를 **둘 다** 호출해야 한다(SHALL). 한쪽만 호출해서는 안 된다(SHALL NOT).

#### Scenario: fetch 실패 시 두 호출 모두 수행
- **WHEN** Pioneer가 URL fetch에 실패했을 때
- **THEN** `scheduler.SetStatus(url, "fetch_failed", nil)`를 호출한 뒤 `scheduler.RecordFetchError(url, errorKind)`를 호출하고, 다음 `Dequeue`로 진행한다

#### Scenario: snapshot 저장 실패도 실패로 보고
- **WHEN** Pioneer가 HTTP fetch에는 성공했으나 snapshot 저장에 실패했을 때
- **THEN** `scheduler.SetStatus(url, "fetch_failed", nil)` + `scheduler.RecordFetchError(url, "network")`를 호출한 뒤 다음 `Dequeue`로 진행한다 (부분 성공을 `"fetched"`로 보고하지 않는다)

#### Scenario: 한쪽 호출 누락 방지
- **WHEN** Pioneer consumer 코드를 정적 분석할 때
- **THEN** `SetStatus("fetch_failed", ...)`만 호출하고 `RecordFetchError`를 호출하지 않거나 그 반대인 경로가 존재하지 않는다 (두 호출은 함께 이루어져야 한다)

---

### Requirement: Pioneer는 fetch 실패를 errorKind로 분류한다
Pioneer는 `RecordFetchError`의 `errorKind` 인자를 다음 규칙으로 결정해야 한다(SHALL).

| 조건 | errorKind |
|------|-----------|
| HTTP 응답 status 400-499 | `"http_4xx"` |
| HTTP 응답 status 500-599 | `"http_5xx"` |
| `net.Error` 이며 `Timeout() == true` | `"timeout"` |
| 그 외 네트워크/IO 에러, snapshot 저장 실패 | `"network"` |

#### Scenario: HTTP 4xx는 "http_4xx"로 분류
- **WHEN** fetch 결과가 HTTP status 404 또는 403 등 4xx일 때
- **THEN** Pioneer는 `RecordFetchError(url, "http_4xx")`를 호출한다 (scheduler 구현체가 즉시 `fetch_error_count = 5`로 dead 처리)

#### Scenario: HTTP 5xx는 "http_5xx"로 분류
- **WHEN** fetch 결과가 HTTP status 500 또는 503 등 5xx일 때
- **THEN** Pioneer는 `RecordFetchError(url, "http_5xx")`를 호출한다

#### Scenario: timeout은 "timeout"으로 분류
- **WHEN** fetch 중 `net.Error`가 발생하고 `Timeout() == true`일 때
- **THEN** Pioneer는 `RecordFetchError(url, "timeout")`를 호출한다

#### Scenario: 그 외 에러와 snapshot 실패는 "network"로 분류
- **WHEN** DNS 실패, connection reset, TLS 에러, 또는 snapshot 저장 실패 등이 발생했을 때
- **THEN** Pioneer는 `RecordFetchError(url, "network")`를 호출한다

---

### Requirement: Pioneer는 fanout B의 producer이다
Pioneer는 한 번의 fetch 성공에 대해 동일한 `URLScheduler` 인스턴스의 두 큐에 모두 써야 한다(SHALL).

1. **새 링크**: `scheduler.Enqueue(scheduler.QueuePioneer, filteredURLs)` — `pioneer_frontier`에 다음 크롤 대상으로 투입.
2. **원본 URL + snapshot_key**: `scheduler.EnqueueHarvester(url, snapshotKey)` — `harvester_frontier`에 UPSERT.

별도의 ingestor나 외부 큐 파이프라인을 경유해서는 안 된다(SHALL NOT).

#### Scenario: 새 링크는 pioneer_frontier로 Enqueue
- **WHEN** Pioneer가 필터를 통과한 새 링크를 frontier에 투입할 때
- **THEN** `scheduler.Enqueue(scheduler.QueuePioneer, filteredURLs)`를 호출한다 (`harvester_frontier`로 직접 보내지 않는다)

#### Scenario: 원본 URL과 snapshot_key는 harvester_frontier로 EnqueueHarvester
- **WHEN** Pioneer가 fetch를 성공적으로 마치고 snapshot을 저장하여 `snapshot_key`를 얻었을 때
- **THEN** `scheduler.EnqueueHarvester(url, snapshotKey)`를 호출하여 `harvester_frontier`에 UPSERT한다

#### Scenario: 이미 harvest된 URL은 EnqueueHarvester가 no-op이다
- **WHEN** `harvester_frontier`에 이미 `harvested_at IS NOT NULL`인 동일 `url_hash` 레코드가 있는 상태에서 Pioneer가 `EnqueueHarvester`를 호출할 때
- **THEN** `ON CONFLICT (url_hash) DO UPDATE ... WHERE harvested_at IS NULL` 가드에 의해 `snapshot_key` / `next_harvest_at` / `harvest_error_count`가 변경되지 않는다 (재harvest 방지)

#### Scenario: 중복 URL은 scheduler가 흡수한다
- **WHEN** 이미 `pioneer_frontier`에 존재하는 URL을 포함한 링크 목록을 Pioneer가 `Enqueue`할 때
- **THEN** 중복은 `pioneer_frontier`의 `UNIQUE(url_hash)` 제약이 걸러내며, Pioneer가 사전 dedup을 수행할 필요가 없다 (링크 필터 체인의 `DedupFilter`와는 별개의 DB 레벨 가드)

---

### Requirement: FilterChain 호출은 Pioneer의 책임이다
Pioneer consumer는 링크 추출 직후, `Enqueue(QueuePioneer, ...)` 직전에 `FilterChain.Apply(links)`를 호출해야 한다(SHALL). 필터 체인의 **구성**(어떤 필터가 어떤 순서로 배치되는지, 각 필터의 정책)은 `pioneer-link-filter-policy`가 정의하며, 본 requirement는 **호출 타이밍**만 규범화한다.

Pioneer consumer는 `FilterChain.Apply`가 반환한 링크 구조체 컬렉션에서 URL 문자열만 추출하여 `scheduler.Enqueue(QueuePioneer, urls...)`에 전달해야 한다(SHALL). baseline scheduler `Enqueue` 시그니처는 `urls ...string`을 받으므로, 링크 구조체의 URL 필드 이외 메타데이터는 Enqueue 경로에서 소모되지 않는다.

#### Scenario: FilterChain.Apply는 Pioneer consumer가 호출한다
- **WHEN** Pioneer가 한 URL에서 링크를 추출했을 때
- **THEN** Pioneer consumer 코드가 `filterChain.Apply(extractedLinks)`를 호출하며, 이 호출을 scheduler나 Enqueue 내부에 위임하지 않는다

#### Scenario: 필터 통과 링크만 Enqueue
- **WHEN** FilterChain이 일부 링크를 제외하고 부분 집합을 반환했을 때
- **THEN** Pioneer는 반환된 부분 집합의 URL 문자열만 `scheduler.Enqueue(QueuePioneer, urls...)`에 전달한다 (제외된 링크는 frontier에 들어가지 않는다)

#### Scenario: Enqueue 인자는 URL 문자열이다
- **WHEN** Pioneer consumer가 `scheduler.Enqueue(QueuePioneer, ...)`를 호출할 때
- **THEN** 전달 인자는 FilterChain 결과 링크의 URL 필드에서 뽑은 문자열 목록이며, 링크 구조체 자체나 메타데이터는 Enqueue 경로에 전달되지 않는다

---

### Requirement: 콘텐츠 추출은 Pioneer의 책임이 아니다
Pioneer는 fetch된 HTML에서 **링크만** 추출해야 하며 snapshot을 저장한 뒤 `harvester_frontier`로 fanout해야 한다(SHALL). JavaScript 스크립트 실행, 미디어 다운로드, Pin 생성, 콘텐츠 항목 배열 생성은 Pioneer의 책임이 아니다(SHALL NOT).

#### Scenario: Pioneer는 링크 목록만 반환한다
- **WHEN** Pioneer가 HTML을 parse할 때
- **THEN** 추출 결과는 URL 목록(및 필터에서 소비할 메타데이터에 한함)이며, 콘텐츠 항목(title/mediaURL/mediaType 등)을 반환하지 않는다

#### Scenario: 스크립트 실행기 미호출
- **WHEN** Pioneer 실행 경로를 분석할 때
- **THEN** Pioneer는 JavaScript 파싱 스크립트 실행기나 처리 파이프라인(Harvester pipeline)을 호출하지 않는다

#### Scenario: Pin 생성 미호출
- **WHEN** Pioneer가 페이지를 처리할 때
- **THEN** Pioneer는 Pin을 생성하지 않으며 `SetStatus` 호출 시 `pinIDs` 인자는 항상 `nil`이다

---

### Requirement: 다중 워커 정확성은 scheduler가 보장한다
Pioneer는 복수 인스턴스로 동시 실행되는 것을 전제로 해야 하며(SHALL), 동일 URL 중복 fetch 방지와 정확히-한 번 claim 보장은 `URLScheduler` 구현체(Postgres `FOR UPDATE SKIP LOCKED` + host token bucket + `next_fetch_at` lease)에 위임해야 한다(SHALL). Pioneer 코드에 워커 간 동시성 제어(분산 락, advisory lock, 중앙 조정자)를 두어서는 안 된다(SHALL NOT).

#### Scenario: 복수 Pioneer가 동일 frontier를 공유
- **WHEN** N개의 Pioneer 인스턴스가 동일 scheduler에 연결되어 실행될 때
- **THEN** 모든 인스턴스는 `Dequeue`/`Enqueue`/`EnqueueHarvester`/`SetStatus`/`RecordFetchError`만으로 동작하며, 서로를 인지하거나 조율하는 코드가 존재하지 않는다

#### Scenario: 동일 URL 동시 fetch 금지는 scheduler의 claim으로 보장
- **WHEN** 동일 URL이 두 Pioneer에게 동시에 요구될 때
- **THEN** scheduler의 claim(`FOR UPDATE SKIP LOCKED` + `next_fetch_at` lease 10분)이 한 쪽에만 URL을 반환하며, Pioneer는 이 동작을 재구현하지 않는다

#### Scenario: Pioneer 내부에 분산 락/mutex 부재
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** 워커 간 조율을 위한 분산 락, advisory lock, Redis lock, 전역 mutex 등이 존재하지 않는다

---

### Requirement: Pioneer는 인메모리 크롤 상태를 보유하지 않는다
Pioneer 프로세스는 인메모리에 URL 큐, visited 집합, 사이트/세션 카운터 같은 **크롤 상태**를 보유해서는 안 된다(SHALL NOT). HTTP 커넥션 풀, DNS 캐시, FilterChain 내부의 robots.txt 캐시 등 프로세스 로컬 리소스 캐시는 이 제약에서 제외된다.

#### Scenario: 재시작 후 상태가 frontier로부터 복구된다
- **WHEN** 크롤 진행 중 Pioneer 프로세스가 종료되었다가 재시작될 때
- **THEN** 새 프로세스는 즉시 `scheduler.Dequeue(scheduler.QueuePioneer)`를 호출하여 frontier의 현재 상태 그대로 크롤을 이어간다 (이전 세션의 큐/visited 복원이 불필요하다)

#### Scenario: visited 맵/세션 카운터 부재
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** URL 방문 여부를 기록하는 인메모리 맵, 사이트별 처리 카운터, 세션 단위 상태 변수가 존재하지 않는다

#### Scenario: 사이트 경계를 기억하지 않는다
- **WHEN** Pioneer가 교차 도메인 링크를 `Enqueue`할 때
- **THEN** Pioneer는 "어느 사이트 세션에 속한다"는 인메모리 맥락 없이 단순히 scheduler에 투입하며, 사이트 분류는 frontier/필터가 담당한다

---

### Requirement: Pioneer는 교차 사이트 크롤을 허용한다
Pioneer는 사이트/도메인 경계를 인지하지 않아야 한다(SHALL NOT). 도메인 제한이 필요한 경우 link filter 정책으로 표현되어야 하며(SHALL), Pioneer의 구조적 제약으로 두지 않는다.

#### Scenario: 외부 도메인 링크 Enqueue 허용
- **WHEN** Pioneer가 현재 fetch 중인 페이지와 다른 도메인의 링크를 추출했을 때
- **THEN** 도메인 차이만을 이유로 해당 링크를 버리지 않으며, FilterChain이 별도로 차단하지 않는 한 `scheduler.Enqueue(QueuePioneer, ...)`에 포함시킨다

#### Scenario: 도메인 제한은 필터의 책임
- **WHEN** 특정 배포에서 도메인 제한이 필요할 때
- **THEN** 제한은 link filter 정책(`pioneer-link-filter-policy`의 `DomainFilter` 등)으로 구현되며 Pioneer 코드 자체는 변경되지 않는다

### Requirement: Pioneer 워커는 성공 Dequeue 100회 후 종료한다
Pioneer 워커 프로세스는 `URLScheduler.Dequeue` 호출을 통해 URL을 최대 100회까지 수령하여 각 URL의 처리 사이클을 완료한 뒤 정상 종료(exit code 0)해야 한다(SHALL). 카운터는 "URL을 실제로 반환한 Dequeue 호출"만 증가시켜야 하며(SHALL), 증가 시점은 **성공 Dequeue 직후**(= 호출이 URL을 리턴한 직후)여야 한다(SHALL). 빈 결과 또는 오류를 반환한 Dequeue 호출은 카운트하지 않아야 한다(SHALL NOT). budget 값(100)은 **빌드 타임 상수**로 고정되어야 하며(SHALL), 환경변수·설정 파일·CLI 플래그 등 어떤 런타임 수단으로도 변경 가능하게 노출되어서는 안 된다(SHALL NOT). ctx 취소 등 외부 종료 신호로 워커가 100회 미만에서 종료되는 경로는 본 정책과 독립적이며, budget은 상한(상향 제한)으로 기능한다. 본 정책은 `harvester-worker-budget`과 대칭이다.

#### Scenario: 99회까지는 종료하지 않는다
- **WHEN** Pioneer 워커가 `URLScheduler.Dequeue`로부터 URL을 99회 수령하여 각 URL의 fetch/링크 추출/Enqueue/SetStatus 사이클을 모두 완료한 직후
- **THEN** 워커는 종료하지 않고 다음 Dequeue를 호출한다.

#### Scenario: 100회째 Dequeue로 받은 URL 처리 완료 후 exit 0
- **WHEN** Pioneer 워커가 100회째 Dequeue로 URL을 수령하여 fetch → 링크 추출 → Enqueue(신규 링크) → SetStatus(frontier 갱신)까지 모두 완료했을 때
- **THEN** 워커 프로세스는 추가 Dequeue를 시도하지 않고 exit code 0으로 종료한다.

#### Scenario: ctx 취소 경로는 budget과 독립적이다
- **WHEN** 100회에 도달하기 전에 외부 ctx 취소 또는 SIGTERM으로 워커가 종료 신호를 받았을 때
- **THEN** 워커는 budget 미소진 상태에서도 종료할 수 있으며(진행 중 fetch는 ctx 전파로 중단될 수 있음), budget 정책은 위반되지 않는다 (budget은 상향 제한이지 하한이 아니다).

#### Scenario: 빈 Dequeue는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue`가 URL을 반환하지 않을 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 다음 Dequeue를 시도한다.

#### Scenario: Dequeue 자체 오류는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue` 호출이 (URL을 반환하지 않고) 오류를 반환할 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 오류를 로깅한 뒤 다시 Dequeue를 시도한다.

#### Scenario: 카운터는 성공 Dequeue 직후 증가한다
- **WHEN** `URLScheduler.Dequeue`가 URL을 성공적으로 반환한 직후
- **THEN** Dequeue 카운터가 1 증가한 뒤에 해당 URL의 fetch 파이프라인이 시작된다.

#### Scenario: budget은 빌드 시 상수
- **WHEN** 운영자가 환경변수나 설정 파일, CLI 플래그로 budget 값을 변경하려 할 때
- **THEN** 워커 동작은 영향을 받지 않으며, 항상 성공 Dequeue 100회 후 종료한다.

---

### Requirement: 진행 중 URL 처리는 중단 없이 완료한다
budget 종료 판정(루프 break 결정)은 **현재 URL 처리 사이클이 완료된 뒤**에만 수행해야 한다(SHALL). 카운터 증가 시점은 별도 Requirement("카운터는 성공 Dequeue 직후 증가한다")가 정의한다. 100회째 Dequeue로 받은 URL의 fetch, 링크 추출, Enqueue(신규 링크 재투입), SetStatus(frontier 갱신)가 모두 끝날 때까지 워커는 종료를 지연해야 한다(SHALL). 진행 중 작업을 중간에 버리고 종료해서는 안 된다(SHALL NOT).

#### Scenario: 100회째 작업이 끝날 때까지 종료 지연
- **WHEN** 카운터가 100에 도달한 뒤에도 현재 URL의 fetch 또는 링크 추출 또는 Enqueue 또는 SetStatus가 아직 진행 중일 때
- **THEN** 워커는 모든 단계를 완료할 때까지 종료하지 않는다.

#### Scenario: 100회째 처리 실패도 정상 종료
- **WHEN** 100회째 URL의 fetch가 오류로 끝나고 frontier에 실패가 기록된 직후
- **THEN** 워커는 exit code 0으로 종료한다 (작업 실패가 워커 종료 코드를 바꾸지 않는다).

#### Scenario: 100회째 완료 후 추가 Dequeue 금지
- **WHEN** 100회째 URL 처리가 완료되었을 때
- **THEN** 워커는 새 Dequeue를 시도하지 않고 즉시 종료 절차에 진입한다.

---

### Requirement: 워커 재시작은 supervisor의 책임이다
Pioneer 워커 프로세스 자체는 자기 자신을 재기동하는 로직을 가져서는 안 된다(SHALL NOT). 종료 후 새 인스턴스를 띄우는 것은 외부 supervisor(systemd, Kubernetes Deployment, Docker restart policy 등)의 책임이어야 한다(SHALL). 종료 직전 워커는 Harvester 워커와 동일한 필드(`reason=budget_exhausted`, `dequeues=100`)를 포함한 기계 파싱 가능한 key=value 포맷 로그(예: `msg="pioneer worker: work budget exhausted" component=pioneer_worker reason=budget_exhausted dequeues=100`)를 정확히 1회 남겨야 한다(SHALL).

#### Scenario: 워커는 자식을 spawn하지 않는다
- **WHEN** Pioneer 워커가 100회 처리를 마치고 종료할 때
- **THEN** 워커는 새 워커 프로세스를 fork/exec하거나 내부 루프를 재개하지 않고 단순히 종료한다.

#### Scenario: 종료 사유 로그
- **WHEN** Pioneer 워커가 budget 소진으로 종료하기 직전일 때
- **THEN** Harvester worker-budget과 동일한 필드(`reason=budget_exhausted`, `dequeues=100`)를 포함한 key=value 포맷 로그 라인(예: `msg="pioneer worker: work budget exhausted" component=pioneer_worker reason=budget_exhausted dequeues=100`)이 정확히 1회 출력된다.

#### Scenario: supervisor가 새 워커를 띄운다
- **WHEN** supervisor(예: docker restart policy, systemd `Restart=always`, k8s `restartPolicy: Always`)가 exit 0로 종료된 Pioneer 워커를 감지할 때
- **THEN** supervisor의 재시작 정책에 따라 새 워커 프로세스가 기동되며, 새 워커는 자체 카운터를 0부터 시작한다.

#### Scenario: 종료 시 상태 청산
- **WHEN** 워커가 budget 소진으로 종료할 때
- **THEN** 인메모리 visited 맵·큐·기타 세션 상태는 프로세스 종료와 함께 폐기되며 외부로 전달되지 않는다.

---

### Requirement: Dequeue 카운터는 워커 간 공유 상태가 아니다
Dequeue 카운터는 각 Pioneer 워커 프로세스의 인메모리 변수로만 보관되어야 하며(SHALL), DB·Redis·frontier 등 워커 간 공유 저장소에 보관해서는 안 된다(SHALL NOT). 본 카운터는 워커 수명 관리용이며, 도메인 상태가 아니다.

#### Scenario: 복수 워커는 각자 독립 카운터를 갖는다
- **WHEN** Pioneer 워커 두 인스턴스가 동시에 실행되고 있을 때
- **THEN** 한 워커가 50회를 처리해도 다른 워커의 카운터에는 영향을 주지 않는다.

#### Scenario: 카운터는 영속되지 않는다
- **WHEN** Pioneer 워커가 종료된 직후
- **THEN** 카운터 값은 어디에도 저장되지 않으며, 새로 기동한 워커는 0에서 다시 시작한다.

---

