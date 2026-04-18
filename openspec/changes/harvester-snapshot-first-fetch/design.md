## Context

Fugue 봇 파이프라인은 Pioneer(그래프 탐색)와 Harvester(노드 콘텐츠 추출)로 분리되어 있다. Pioneer는 BFS 탐색 중 원본 HTML을 fetch하면서 ObjectStorage에 스냅샷을 적재한다(키 규약·TTL 365일·gzip 압축·sha256 해시 함수는 `pioneer-snapshot-storage`가 정의). 현재 Harvester는 노드 처리 시점에 원본 URL을 다시 HTTP로 호출하므로 다음 문제가 발생한다.

- 동일 사이트에 Pioneer + Harvester 두 번 트래픽이 발생해 robots/rate-limit 부담이 커진다.
- Pioneer 시점과 Harvester 시점 사이에 원본이 변경/삭제되면 그래프와 콘텐츠가 불일치한다.
- 외부 사이트의 일시적 4xx/5xx가 Harvester 단계까지 그대로 노출된다.

참고 의사코드는 `apps/api/fuguebot_pseudo.go` 라인 86-97의 `CompositeFetcher`다. ObjectStorage를 우선 시도하고 실패 시 HTTP로 폴백하는 단순한 합성 패턴을 채택한다.

## Goals / Non-Goals

**Goals:**
- Harvester의 fetch 경로를 `CompositeFetcher` 의미론(ObjectStorage 우선 → HTTP fallback)으로 명세화한다.
- 스냅샷 hit 정상 경로에서 Harvester가 외부 네트워크를 호출하지 않도록 보장한다.
- 스냅샷 miss/expired/조회 실패 시 HTTP 폴백 동작과, 두 경로 모두 실패 시 실패 카운터(`harvest_error_count`) 증가를 명세화한다.
- 키 포맷/해시 함수 정의는 본 change에서 중복 기술하지 않고 `pioneer-snapshot-storage` 참조로 통일한다.

**Non-Goals:**
- ObjectStorage 쓰기 경로(저장 키, TTL=365d, gzip 압축, 멱등성, sha256 해시 계산)는 본 변경에서 정의하지 않는다. → `pioneer-snapshot-storage`.
- HTTP fallback 후 결과를 ObjectStorage에 재저장하는 정책은 본 변경 범위 밖이다.
- Fetcher 단의 retry/backoff 횟수·간격은 본 변경에서 정의하지 않는다. → `scheduler-retry-backoff`.
- ObjectStorage 백엔드 선택(S3/MinIO 등) 및 자격 증명 관리.

## Decisions

### Decision 1: Harvester는 단일 `Fetcher` 인터페이스에 의존하고, 구현체로 `CompositeFetcher`를 주입한다

Harvester는 fetch 출처(스냅샷/HTTP)를 알 필요가 없으며, `Fetcher.Fetch(url) ([]byte, error)` 시그니처만 호출한다. ObjectStorage 우선 → HTTP fallback 합성은 `CompositeFetcher`가 캡슐화한다.

- **이유**: Harvester 코드는 출처와 무관하게 동일한 파싱 파이프라인을 수행하며, fetch 정책 변화(예: 향후 캐시 계층 추가)를 Harvester 변경 없이 흡수할 수 있다.
- **대안**: Harvester가 직접 ObjectStorage/HTTP 두 클라이언트를 호출하고 분기 처리. → Harvester가 저장소 세부사항을 알게 되어 결합도가 증가하고, Pioneer와의 fetch 로직 공유(기존 bot spec 요구)와 어긋난다.

### Decision 2: ObjectStorage 조회 실패는 모두 "miss"로 간주하고 HTTP fallback으로 진행한다

ObjectStorage가 반환할 수 있는 모든 실패 케이스 — **키 없음(not found), TTL 만료(expired), 네트워크 에러, 권한 에러(403/AccessDenied), 내부 에러(5xx) 포함** — 를 구분하지 않고 `CompositeFetcher` 관점에서는 모두 단일 "스냅샷 사용 불가(miss)"로 동일 처리해 HTTP fallback으로 라우팅한다. 의사코드도 단일 `err != nil` 분기만 둔다(라인 92-95).

- **이유**: Harvester의 정상 경로(파싱)를 보장하기 위해 가용한 폴백 경로(HTTP)를 우선 시도하는 것이 사용자 가치(콘텐츠 노출)에 부합한다. 실패 원인별 분기를 fetch 로직에 넣으면 코드 복잡도가 커지고, ObjectStorage 일시 장애가 Harvester 가용성을 떨어뜨린다.
- **에러 종류 구분은 로그 레벨에서만 수행한다**: `ObjectStorageFetcher`는 내부적으로 `not_found` / `expired` / `network` / `permission` / `internal` 등 에러 종류를 분류해 로그/메트릭으로 남긴다(운영 분석·알람 임계치 산정용). 단, 이 구분은 fetch 의사결정(fallback 여부)에는 일체 영향을 주지 않는다. 즉 동작(behavior)이 아니라 관측(observability)의 영역이다.
- **대안 1**: 권한/네트워크 에러는 즉시 실패 처리. → ObjectStorage 일시 장애가 Harvester 전체 가용성을 떨어뜨린다. 기각.
- **대안 2**: 만료(expired)는 HTTP fetch + 재저장으로 자가 갱신. → 스냅샷 쓰기 경로를 본 변경에 끌어들이게 되어 범위를 벗어난다. 재갱신은 향후 별도 change에서 다룰 수 있다.

### Decision 3: ObjectStorage와 HTTP 둘 다 실패하면 노드 단위 실패로 분류하고 `harvest_error_count`를 증가시킨다

`CompositeFetcher.Fetch`가 최종적으로 에러를 반환하면 Harvester는 해당 노드 처리를 중단하고 실행 통계의 실패 카운터를 1 증가시킨다. 다른 노드 처리에는 영향을 주지 않는다.

- **이유**: 기존 bot spec의 "Harvester 실행 완료 시 전체 통계를 집계한다" 요구사항과 정합. 이중 실패는 Harvester가 자체 복구할 수 없는 상태이며, 가시성 확보가 우선이다.
- **대안**: 이중 실패 시 우선순위 큐로 재투입. → retry/backoff 정책의 일부이므로 `scheduler-retry-backoff` 범위.

### Decision 4: 스냅샷 본문은 HTTP 응답 본문과 동일하게 취급한다(파싱 파이프라인 무변경)

`CompositeFetcher.Fetch`가 반환하는 `[]byte`는 출처와 무관하게 "원본 HTML 바이트열"의 의미를 가진다. **gzip 해제는 `ObjectStorageFetcher` 내부 책임**이며(즉 저장 포맷 처리는 Fetcher 경계 안에서 완결된다) Harvester 파이프라인에는 압축되지 않은 원본 HTML 바이트열만 노출된다.

- **이유**: 파서/스크립트 실행기가 출처별로 다른 코드 경로를 갖지 않게 하여 결정론을 보장한다.
- **참고**: 저장 포맷(gzip)·키 규약·sha256 해시 함수는 `pioneer-snapshot-storage`가 정의하므로 본 변경은 의미론(`Fetch`가 동일한 바이트열을 반환)만 명세한다.

### Decision 5: 스냅샷 키 포맷과 해시 함수는 `pioneer-snapshot-storage`를 참조한다 (자체 정의 금지)

`ObjectStorageFetcher`가 조회할 스냅샷 키를 계산할 때 본 change는 자체 키 포맷을 기술하지 않는다. 대신 `pioneer-snapshot-storage` capability가 제공하는 키 빌더(예: `SnapshotKey(normalizedURL, t)` 함수)를 **그대로 import해 재사용**한다. 해시 함수는 **sha256** 으로 pioneer와 일치시키며, Harvester 측에서 재구현하지 않는다.

- **이유**: Pioneer가 쓰는 키와 Harvester가 읽는 키가 불일치하면 전체 구조가 붕괴한다. 단일 공용 함수에 소스를 집중시키는 것이 정합성 보증의 유일한 수단이다.
- **대안**: 본 change에서 키 포맷을 문서화 목적으로 중복 기술. → DECISIONS.md §7에서 "자체 기술하지 않고 참조만 한다"로 확정됨. 중복 기술은 향후 divergence 리스크.

## Risks / Trade-offs

- **[리스크] 스냅샷 staleness**: TTL 365일 동안 원본이 변경되어도 Harvester는 옛 스냅샷으로 파싱한다. → 본 변경 범위 밖이지만, 향후 노드 단위 재크롤 트리거(invalidate)를 별도 change로 도입할 수 있다.
- **[리스크] snapshot TTL 경과 후 stale snapshot 처리**: ObjectStorage lifecycle rule이 365일 경과 객체를 자동 삭제하므로, 기한이 지난 스냅샷은 조회 시점에 자연스럽게 "키 없음(not found)"으로 반환된다. 이는 Decision 2에 의해 단일 "miss"로 취급되어 HTTP fallback을 통해 최신 HTML을 가져오는 경로로 수렴한다. 즉 TTL 만료는 fetch 경로의 자기 복구 메커니즘과 이미 정합하며, Harvester 측 추가 처리는 필요하지 않다.
- **[리스크] ObjectStorage 가용성 저하 시 HTTP 트래픽 폭증**: 모든 Harvester 호출이 폴백되면 외부 트래픽이 평소의 N배가 된다. → robots/rate 제어는 `scheduler-host-token-bucket`이 흡수한다는 전제. 본 변경은 fetcher 의미론만 정의한다.
- **[트레이드오프] 결정론 ↔ 신선도**: 스냅샷 우선은 결정론을 얻는 대신 신선도를 잃는다. MVP 단계에서는 결정론과 외부 트래픽 절감이 우선이다.
- **[리스크] 이중 실패 노이즈**: 일시적 네트워크 장애로 ObjectStorage·HTTP가 동시에 실패할 때 `harvest_error_count`가 과대 집계될 수 있다. → 본 변경은 카운터 증가만 정의하며, 알람 임계치는 운영 설정에서 별도 관리한다. 로그에 남긴 ObjectStorage 실패 종류(not_found/expired/network/permission/internal)를 활용해 원인 분석이 가능하다.
- **[리스크] pioneer-snapshot-storage 공용 함수 변경**: 키 빌더/해시 함수 시그니처가 바뀌면 Harvester 읽기 측도 함께 갱신해야 한다. → 두 change 간 코드 공유가 강제되므로 컴파일 단계에서 조기 발견된다. Task 5.2로 정합성을 명시적으로 검증한다.
