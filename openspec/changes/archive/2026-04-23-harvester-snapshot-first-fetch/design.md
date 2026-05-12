## Context

Fugue 봇 파이프라인은 Pioneer(그래프 탐색)와 Harvester(노드 콘텐츠 추출)로 분리되어 있다. Pioneer는 BFS 탐색 중 원본 HTML을 fetch하면서 ObjectStorage에 스냅샷을 적재한다(키 규약·TTL 365일·압축 저장·해시 함수는 `bot` capability의 스냅샷 쓰기 Requirement에서 정의 — 구 `pioneer-snapshot-storage` change의 결과가 `bot` capability에 흡수되었다. 공용 코드는 `apps/api/internal/bot/snapshot` 패키지의 `SnapshotKey`, `HashNormalizedURL`, `SnapshotKeyPattern` 세 심볼). 현재 Harvester는 노드 처리 시점에 원본 URL을 다시 HTTP로 호출하므로 다음 문제가 발생한다.

- 동일 사이트에 Pioneer + Harvester 두 번 트래픽이 발생해 robots/rate-limit 부담이 커진다.
- Pioneer 시점과 Harvester 시점 사이에 원본이 변경/삭제되면 그래프와 콘텐츠가 불일치한다.
- 외부 사이트의 일시적 4xx/5xx가 Harvester 단계까지 그대로 노출된다.

참고 의사코드는 `apps/api/fuguebot_pseudo.go` 라인 97–112의 `CompositeFetcher`다. ObjectStorage를 우선 시도하고 실패 시 HTTP로 폴백하는 단순한 합성 패턴을 채택한다.

## Goals / Non-Goals

**Goals:**
- Harvester의 fetch 경로를 "ObjectStorage 우선 → HTTP fallback" 합성 의미론으로 명세화한다.
- 스냅샷 hit 정상 경로에서 Harvester가 외부 네트워크를 호출하지 않도록 보장한다.
- 스냅샷 사용 불가 시 HTTP 폴백 동작과, 두 경로 모두 실패 시 실행 통계의 fetch 실패 집계 증가를 명세화한다.
- 키 포맷/해시 함수 정의는 본 change에서 중복 기술하지 않고 `bot` capability의 스냅샷 쓰기 경로가 확정한 공용 함수를 import해 재사용한다.

**Non-Goals:**
- ObjectStorage 쓰기 경로(저장 키, TTL=365d, 압축 저장, 멱등성, 해시 계산)는 본 변경에서 정의하지 않는다. → `bot` capability의 스냅샷 쓰기 Requirement(구 `pioneer-snapshot-storage` change로 확정).
- HTTP fallback 후 결과를 ObjectStorage에 재저장하는 정책은 본 변경 범위 밖이다.
- Fetcher 단의 retry/backoff 횟수·간격은 본 변경에서 정의하지 않는다. → `scheduler-retry-backoff`.
- Consumer 경계의 입력 형식은 본 변경 범위 밖이며, sibling change `harvester-snapshot-first-fetchconsumer`가 `(ctx, url)` 단일 형태(snapshot_key 비전달)로 **신규 확정**한다. 본 change는 해당 계약을 **의존**한다(기존 상태의 유지가 아님).
- ObjectStorage 백엔드 선택(S3/MinIO 등) 및 자격 증명 관리.

## Decisions

### Decision 1: Harvester는 단일 `Fetcher` 인터페이스에 의존하고, 구현체로 `CompositeFetcher`를 주입한다

Harvester는 fetch 출처(스냅샷/HTTP)를 알 필요가 없으며, 공통 `Fetcher` 인터페이스에만 의존한다. ObjectStorage 우선 → HTTP fallback 합성은 `CompositeFetcher`가 캡슐화한다. 구현 시그니처는 `Fetch(url string) ([]byte, error)`로 한다.

- **이유**: Harvester 코드는 출처와 무관하게 동일한 파싱 파이프라인을 수행하며, fetch 정책 변화(예: 향후 캐시 계층 추가)를 Harvester 변경 없이 흡수할 수 있다.
- **대안**: Harvester가 직접 ObjectStorage/HTTP 두 클라이언트를 호출하고 분기 처리. → Harvester가 저장소 세부사항을 알게 되어 결합도가 증가하고, Pioneer와의 HTTP 경계 설정 공유(기존 bot spec 요구)와 어긋난다.

**두 계층의 경계 명시 — 저수준 `Fetcher` vs snapshot-first 진입점**: 본 change의 `Fetcher` 인터페이스(`Fetch(url) ([]byte, error)`)는 **저수준 바이트열 리더** 계약이며, sibling change `harvester-snapshot-first-fetchconsumer`가 정의하는 **snapshot-first 진입점**(`(ctx, url) → (html, errorKind, err)` 3-tuple)과는 별개 계층이다. 양자 사이의 어댑터(저수준 `error` → `errorKind` 4종 분류 매핑)는 **sibling change의 구현 범위**에 속하며, 본 change의 행위 계약은 어댑터 아래의 바이트열 반환 의미론만 고정한다. 본 Requirement 본문에서 "단일 fetch 진입점"은 저수준 `Fetcher` 계층을 지칭한다.

### Decision 1a: Fetcher 시그니처에 `context.Context`를 포함시키지 않는다

`Fetcher.Fetch`는 의도적으로 `ctx`를 받지 않는다. 상위(consumer) 경계에는 ctx가 존재하지만 fetcher 단으로 전파하지 않는다.

- **이유**: 의사코드(`apps/api/fuguebot_pseudo.go` 97–112)와 일치시켜 최소 인터페이스를 유지. 각 구현체는 자체 타임아웃(`bot` capability가 정의한 HTTP 경계 설정의 타임아웃, ObjectStorage SDK의 요청 타임아웃)으로 경계 시간을 보장한다. 구체 수치는 해당 Requirement 및 구현 설정에서 관리하며 본 change 행위 계약의 대상이 아니다.
- **트레이드오프**: consumer의 ctx 취소가 fetcher 내부 요청을 즉시 중단시키지 못한다. 실제로는 내부 타임아웃이 수초 내에 해제하므로 허용 가능한 지연. 취소 전파가 필요한 경우 별도 change에서 시그니처를 확장한다(반환 의미론은 Decision 4에서 별도 고정).
- **sibling change의 `errorKind="timeout"`과의 관계**: sibling change가 요구하는 `"timeout"` 분류는 **HTTP helper(또는 ObjectStorage SDK) 자체 타임아웃 트리거 경로**로 충족된다. consumer ctx 취소는 본 Fetcher 경계에 전파되지 않으며, consumer 측에서 별도로 관찰·처리한다(`errorKind="timeout"`로 집계되지 않을 수 있다). 즉 두 change가 말하는 "timeout"은 동일 대상이 아니라 계층이 다르며, 본 change는 내부 타임아웃 경로만을 `"timeout"`의 원천으로 본다.
- **sibling change의 그 밖의 `errorKind`(`network`/`http_4xx`/`http_5xx`)와의 관계**: 본 change의 `Fetcher`가 반환하는 `error`는 **분류되지 않은 단일 타입**이며, 4종 `errorKind`로의 매핑(DNS 실패·연결 거부 → `network`, HTTP 응답 4xx/5xx → `http_4xx`/`http_5xx`)은 전부 sibling change의 어댑터 구현 책임이다. 본 change의 행위 계약은 어댑터 아래의 "바이트열 또는 에러" 리더 의미론까지만 고정한다.

### Decision 1b: Pioneer와 Harvester의 fetch 시그니처는 서로 다를 수 있다

`bot` capability의 기존 Scenario "Pioneer와 Harvester의 fetch 로직 공유"는 "동일한 공유 함수"를 요구했으나, 본 change의 MODIFIED 델타는 이를 "HTTP helper 수준의 공유"로 완화한다.

- **이유**: Pioneer는 fetch 결과를 ObjectStorage에 **쓰는** 주체로 `(bytes, finalURL, statusCode, err)` 같은 풍부한 반환값이 필요하다(status code 기반 `RecordFetchError` kind 분류). Harvester는 스냅샷 **읽기** 측으로 `([]byte, error)`의 단순 리더면 충분하다. 두 역할을 동일 함수 시그니처에 묶으면 Pioneer 측 정보 손실 또는 Harvester 측 불필요한 반환값이 발생한다.
- **공유 대상**: 사이즈 제한·리다이렉트 제한·타임아웃·User-Agent 등 HTTP 경계 설정. 이는 `apps/api/internal/bot` 패키지 내부의 공유 helper(현재 `fetchHTMLShared`) 함수를 Pioneer/Harvester 양쪽이 호출하는 방식으로 달성한다. helper의 파일 배치는 구현 세부이며 행위 계약의 대상이 아니다.
- **대안**: 동일 시그니처를 강제. → Pioneer의 에러 분류가 약화되거나 Harvester가 불필요한 반환값을 무시해야 한다. 기각.

### Decision 1c: Fetch 진입점은 URL 종류를 구분하지 않는다

기존 `bot` capability Scenario "Harvester HTML 가져오기"는 WHEN을 "노드의 sample_url 또는 template url로 HTML을 요청할 때"로 한정했다. MODIFIED 델타는 이 조건을 "노드 URL에 대해 fetch를 요청"으로 일반화한다.

- **이유**: `sample_url` vs `template url` 선택은 **Harvester 상위 경로(pin pipeline)** 의 결정이며, fetch 진입점은 어떤 URL이든 동일한 합성 의미론으로 처리한다. 진입점 경계에서 URL 종류를 구분하면 `(ctx, url)`이라는 consumer 계약(sibling change)이 깨진다.
- **호환성**: Harvester 상위 경로는 여전히 sample_url/template url을 상황에 맞게 선택해 진입점에 넘긴다. 즉 외부 관찰 가능한 URL 종류 사용 패턴은 보존되며, 행위 계약의 경계만 정리한다.

### Decision 2: ObjectStorage 조회 실패는 모두 "사용 불가"로 간주하고 HTTP fallback으로 진행한다

ObjectStorage가 반환할 수 있는 모든 실패 케이스 — 키 없음, 네트워크 에러, 권한 에러, 내부 에러(5xx) 등 — 를 구분하지 않고 단일 "사용 불가(miss)"로 동일 처리해 HTTP fallback으로 라우팅한다. TTL 만료는 lifecycle 삭제에 의해 "키 없음"으로 수렴하므로 독립 sentinel이나 관측 범주가 아니다. 의사코드도 단일 `err != nil` 분기만 둔다(라인 107–110).

- **이유**: Harvester의 정상 경로(파싱)를 보장하기 위해 가용한 폴백 경로(HTTP)를 우선 시도하는 것이 사용자 가치(콘텐츠 노출)에 부합한다. 실패 원인별 분기를 fetch 로직에 넣으면 코드 복잡도가 커지고, ObjectStorage 일시 장애가 Harvester 가용성을 떨어뜨린다.
- **에러 종류 구분은 로그 레벨에서만 수행한다**: `ObjectStorageFetcher` 구현은 내부적으로 실패 종류를 분류해 로그/메트릭으로 남긴다(운영 분석·알람 임계치 산정용). 관측 라벨의 구체적 문자열 집합은 행위 계약이 아니라 운영 설정 영역이며, 본 spec의 행위 요구사항은 "실패가 로그로 식별 가능하다"까지만이다.
- **대안 1**: 권한/네트워크 에러는 즉시 실패 처리. → ObjectStorage 일시 장애가 Harvester 전체 가용성을 떨어뜨린다. 기각.
- **대안 2**: 만료(expired)는 HTTP fetch + 재저장으로 자가 갱신. → 스냅샷 쓰기 경로를 본 변경에 끌어들이게 되어 범위를 벗어난다. 재갱신은 향후 별도 change에서 다룰 수 있다.

### Decision 3: ObjectStorage와 HTTP 둘 다 실패하면 노드 단위 실패로 분류하고 실행 통계의 fetch 실패 카운터를 증가시킨다

`CompositeFetcher.Fetch`가 최종적으로 에러를 반환하면 Harvester는 해당 노드 처리를 중단하고 **워커 프로세스 내 실행 통계(in-memory 카운터)** 의 fetch 실패 카운터를 1 증가시킨다. 다른 노드 처리에는 영향을 주지 않는다. 내부 식별자는 구현 문서에서 관리하며 spec은 관찰 가능한 1 증가만 요구한다.

**`harvester_frontier.harvest_error_count` DB 컬럼과의 구분**: scheduler spec이 정의한 `harvester_frontier.harvest_error_count`는 retry/backoff 및 partial index 기준의 DB 컬럼이며, `harvester-scheduler-consumer` capability의 `RecordHarvestError` 경로가 증가시킨다. 본 Decision의 실행 통계 카운터는 워커 실행 메트릭 목적의 별개 in-memory 집계이며, DB 컬럼과 이름이나 증가 경로를 공유하지 않는다.

- **이유**: 기존 bot spec의 "Harvester 실행 완료 시 전체 통계를 집계한다" 요구사항과 정합. 이중 실패는 Harvester가 자체 복구할 수 없는 상태이며, 가시성 확보가 우선이다.
- **대안**: 이중 실패 시 우선순위 큐로 재투입. → retry/backoff 정책의 일부이므로 `scheduler-retry-backoff` 범위.

### Decision 4: 스냅샷 본문은 HTTP 응답 본문과 동일하게 취급한다(파싱 파이프라인 무변경)

`CompositeFetcher.Fetch`가 반환하는 `[]byte`는 출처와 무관하게 "원본 HTML 바이트열"의 의미를 가진다. 저장 포맷 변환(ObjectStorage 경로의 압축 해제 등)은 Fetcher 경계 안에서 완결되며, Harvester 파이프라인에는 원본 HTML 바이트열만 노출된다. HTTP 경로는 애초에 압축 블롭을 받지 않으므로 별도 변환이 필요하지 않다.

- **이유**: 파서/스크립트 실행기가 출처별로 다른 코드 경로를 갖지 않게 하여 결정론을 보장한다.
- **참고**: 저장 포맷(압축)·키 규약·해시 함수는 Pioneer 쓰기 경로가 정의하므로 본 변경은 의미론(`Fetch`가 동일한 바이트열을 반환)만 명세한다.

### Decision 5: 스냅샷 키 포맷과 해시 함수는 Pioneer 쓰기 경로의 공용 함수를 import해 재사용한다 (재정의 금지)

`ObjectStorageFetcher`가 조회할 스냅샷 키를 계산할 때 본 change는 자체 키 포맷을 기술하지 않는다. 대신 `bot` capability 스냅샷 쓰기 경로가 확정한 공용 심볼 — `apps/api/internal/bot/snapshot` 패키지의 `SnapshotKey(normalizedURL string, t time.Time) string`, `HashNormalizedURL(normalizedURL string) string`, 그리고 키 포맷 상수 `SnapshotKeyPattern` — 를 **그대로 import해 재사용**한다. 해시 함수는 해당 패키지의 기본값(sha256 기반)을 따르며 Harvester 측에서 재구현하지 않는다.

**normalized URL 입력 함수도 Pioneer와 동일한 것을 사용한다**: `SnapshotKey(normalizedURL, t)`의 첫 인자는 Pioneer 쓰기 측이 정의한 **동일한 URL 정규화 함수**의 출력을 그대로 사용해야 한다. 정규화 함수가 어긋나면 해시 입력이 달라 키가 비트 단위로 일치하지 않게 되어 Decision 5의 정합성 보장이 깨진다. 공용 키 빌더를 import해도 입력 정규화가 다르면 의미가 없다. (참고 — informative: 본 change 작성 시점의 Pioneer 쓰기 경로 구현은 `urlcanon.Canonical` 계열 함수를 사용한다. 구체 함수 이름은 `bot` capability 스냅샷 쓰기 Requirement의 구현 세부이며, 본 Decision은 그 함수 출력을 재사용한다는 규범적 관계만 고정한다.)

- **이유**: Pioneer가 쓰는 키와 Harvester가 읽는 키가 불일치하면 전체 구조가 붕괴한다. 단일 공용 함수에 소스를 집중시키고, 해시 입력(normalized URL)까지 동일 함수로 맞추는 것이 정합성 보증의 유일한 수단이다.
- **대안**: 본 change에서 키 포맷을 문서화 목적으로 중복 기술. → DECISIONS.md §7에서 "자체 기술하지 않고 참조만 한다"로 확정됨. 중복 기술은 향후 divergence 리스크.

### Decision 5a: 스냅샷 키의 시간 세그먼트는 Harvester 실행 시각 기준 현재 UTC 날짜로 결정한다

`SnapshotKey(normalizedURL, t)`의 `t`는 **각 `Fetch(url)` 호출 진입 시점에 한 번 캡처한 현재 UTC 시각**(`time.Now().UTC()`)을 사용한다. 동일 호출 내에서는 재캡처하지 않는다(이 재캡처 금지는 구현 가이드이며, 외부 관찰 가능한 행위는 "호출당 일자 세그먼트가 단일 값으로 결정된다"로 수렴한다). 즉 "같은 UTC 일자에 Pioneer가 쓴 스냅샷"만 hit 대상이다. 그 외(전일 이전에 쓰인 스냅샷 등)는 Decision 2의 단일 "사용 불가"로 수렴해 HTTP 폴백으로 라우팅된다.

- **이유**:
  - Consumer 경계는 `(ctx, url)`로 고정되며 `snapshot_key`를 인자로 받지 않는다(`harvester-snapshot-first-fetchconsumer` spec이 강제). 따라서 Harvester 읽기 측은 URL로부터 키를 재계산해야 한다.
  - `time.Now().UTC()`는 추가 상태·DB 접근 없이 결정 가능한 최소 전략이다.
  - 같은 UTC 일자 내에 Pioneer가 enqueue하고 Harvester consumer가 dequeue하는 정상 운영 상황에서는 hit률이 높고, 날짜 경계 부근에서만 miss가 발생한다. miss 시에도 HTTP 폴백이 자연스러운 자기 복구 경로를 제공하므로 동작 정확성은 유지된다.
- **트레이드오프**:
  - UTC 자정 이후 실행되는 Harvester는 전일 Pioneer가 쓴 스냅샷을 읽지 못하고 HTTP 폴백에 의존한다. 이 경우 "외부 트래픽 절감" 목표가 부분적으로만 달성된다.
  - 백로그 누적으로 Harvester가 Pioneer 쓰기보다 하루 이상 지연되면 hit률이 0에 수렴한다. 백로그 관리는 scheduler·consumer 측 책임 영역.
- **대안 1**: 최근 N UTC일자를 순차 probe. → ObjectStorage 호출 횟수가 N배가 되며, miss 케이스에서 응답 지연이 N배로 증가. MVP 단순성을 위해 기각하되 향후 hit률 데이터 확보 후 재검토 가능.
- **대안 2**: `harvester_frontier.snapshot_key` 컬럼을 Harvester 진입점 내부에서 별도 SELECT해 사용. → `harvester-snapshot-first-fetchconsumer` spec이 "진입점 내부에서 pioneer 공용 빌더로 계산한다"로 경계를 고정했고, 추가 DB round-trip이 consumer 루프의 결정론을 약화시킨다. 기각.
- **대안 3**: Scheduler Dequeue 반환에 `snapshot_key`를 포함시켜 전달. → consumer 경계 계약(`(ctx, url)`)을 깨며, sibling change 및 `scheduler` spec 개정까지 필요. 별도 change로 분리.

### Decision 5b: 왜 `harvester_frontier.snapshot_key` 컬럼을 읽지 않는가

`scheduler` spec(`URLScheduler.EnqueueHarvester(url, snapshotKey)`)은 Pioneer가 fetch 직후 `harvester_frontier.snapshot_key`를 원자적으로 적재한다. 이 컬럼이 단일 정합 소스(SSOT)에 가장 가깝다. 그럼에도 본 change는 이 경로를 쓰지 않는다.

- **이유**: Consumer 경계 계약(`harvester-snapshot-first-fetchconsumer` spec)이 이미 "진입점은 `(ctx, url)`만 받으며 snapshot_key를 별도 SELECT하거나 인자로 전달하지 않는다"를 강제한다. `harvester_frontier.snapshot_key`는 향후 `scheduler-harvest-retry`류 후속 change에서 운영 관찰/재시도 힌트로 활용될 여지를 남겨둔 컬럼이며, 본 change의 읽기 경로 SSOT는 아니다.
- **정합성 보장**: Decision 5의 공용 키 빌더 재사용과 Decision 5a의 UTC 일자 고정 전략이 합쳐져, Pioneer 쓰기 키와 Harvester 읽기 키는 "같은 UTC 일자 내"에서 비트 단위로 동일하다. 날짜 경계 밖에서는 miss → HTTP fallback으로 수렴한다.
- **기존 `harvester/spec.md` 311행 문구 해석**: "`harvester-snapshot-first-fetch` capability가 제공하는 snapshot-first 경로… snapshot_key가 있으면 snapshot 우선, miss 시 HTTP live fetch"는 본 change 이후 다음과 같이 재해석된다. (1) 본 change가 아카이브되면 `harvester-snapshot-first-fetch`는 capability가 아니라 완결된 change 이름이며 실제 요구사항은 `bot` capability(`openspec/specs/bot/spec.md`)에 통합된다. (2) "snapshot_key가 있으면"의 판단 주체는 consumer가 아니라 snapshot-first 진입점 내부이며, 진입점은 URL로부터 파생한 키를 내부적으로 시도한 뒤 hit/miss에 따라 snapshot/HTTP live fetch로 분기한다. consumer는 snapshot_key를 인자로 전달하지 않는다(`harvester-snapshot-first-fetchconsumer` spec 강제).
- **문구 정리 시점**: `harvester/spec.md` 원문의 자구 정리(capability 명칭 + snapshot_key 판단 주체 표현)는 후속 스텁 change(가칭 `harvester-mainloop-snapshot-wording-sync`; 최종 이름은 스텁 change 생성 시점에 결정되며, 본 change는 해당 이름을 사전 고정하지 않는다)에서 한 줄 수정으로 동기화한다(tasks.md §6.1 후속 추적). 본 change의 MODIFIED 범위에 포함시키지 않은 이유는 (a) `harvester` capability를 건드리면 consumer/스케줄러 capability 간 재검증을 촉발해 본 change scope가 과도하게 확장되고, (b) 자구 정리가 본 change 구현에 기능적 선행 조건이 아니기 때문이다. archive 사이 단기간의 자구 불일치는 design.md의 본 문단을 통해 공식화된 의도된 trade-off다.
- **후속 스텁 모니터링 책임**: 본 change archive PR을 머지하는 리뷰어(또는 archive 담당 엔지니어)가 동일 스프린트(또는 1주 이내)에 스텁 change가 열렸는지 확인할 1차 책임자다. 해당 기간 내 스텁 change가 열리지 않으면 운영 이슈 트래커(팀 이슈 보드)에 에스컬레이션해 후속 처리한다. CI/lint 레벨의 자동 강제는 본 change 범위 밖이며, 향후 repository-wide linter가 도입되면 해당 linter에 capability 명칭 검증 룰을 추가해 대체한다.
- **향후 확장 경로**: hit률이 운영상 문제로 드러나면 (a) Dequeue 반환 확장, (b) 진입점 시그니처 확장, (c) 최근 N일자 probe — 세 가지 중 데이터를 보고 후속 change로 선택한다.

## Risks / Trade-offs

- **[리스크] 스냅샷 staleness**: TTL 365일 동안 원본이 변경되어도 Harvester는 옛 스냅샷으로 파싱한다. → 본 변경 범위 밖이지만, 향후 노드 단위 재크롤 트리거(invalidate)를 별도 change로 도입할 수 있다.
- **[리스크] TTL 경과 후 stale snapshot 처리**: ObjectStorage lifecycle rule이 기한 경과 객체를 자동 삭제하므로, 기한이 지난 스냅샷은 조회 시점에 자연스럽게 "키 없음"으로 반환된다. 이는 Decision 2에 의해 단일 "사용 불가"로 취급되어 HTTP fallback을 통해 최신 HTML을 가져오는 경로로 수렴한다. Harvester 측 추가 처리는 필요하지 않다.
- **[리스크] ObjectStorage 가용성 저하 시 HTTP 트래픽 폭증**: 모든 Harvester 호출이 폴백되면 외부 트래픽이 평소의 N배가 된다. → robots/rate 제어는 `scheduler-host-token-bucket`이 흡수한다는 전제. 본 변경은 fetcher 의미론만 정의한다.
- **[리스크] UTC 자정 경계 / 백로그 누적**: Decision 5a 트레이드오프로 hit률이 낮아질 수 있다. 자정 근처 시점의 drop-off 및 Pioneer–Harvester 시차별 hit률은 운영 대시보드로 관측해 후속 change 결정의 근거로 삼는다.
- **[리스크] 이중 실패 노이즈**: 일시적 네트워크 장애로 ObjectStorage·HTTP가 동시에 실패할 때 실패 집계가 과대 집계될 수 있다. → 본 변경은 집계 증가 의미만 정의하며, 알람 임계치는 운영 설정에서 별도 관리한다. 로그에 남긴 ObjectStorage 실패 종류를 활용해 원인 분석이 가능하다.
- **[리스크] 공용 키 함수 변경**: 키 빌더/해시 함수 시그니처가 바뀌면 Harvester 읽기 측도 함께 갱신해야 한다. → `apps/api/internal/bot/snapshot` 패키지를 import 경유로만 사용하므로 컴파일 단계에서 조기 발견된다. Task 5.2로 정합성을 자동 검증한다.
