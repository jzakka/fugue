## Context

두 기존 change의 경계가 교차하는 지점을 구체화한다.

- `harvester-scheduler-consumer`는 consumer 루프를 정의하면서 fetch 단계를 `fetchSnapshotOrLive(ctx, url) → (html, errorKind, fetchErr)`로 가정한 의사코드(`design.md:139`)를 제시하지만, 해당 capability spec은 이 진입점의 반환 형태를 행위 계약으로 고정하지 않는다.
- `harvester-snapshot-first-fetch`는 `Fetcher.Fetch(url) ([]byte, error)` 시그니처의 `CompositeFetcher`만 정의하며 consumer가 필요한 `errorKind` 분류 정보를 포함하지 않는다. 같은 change의 Decision 5는 "스냅샷 키는 `ObjectStorageFetcher` 내부에서 `pioneer-snapshot-storage`의 공용 빌더로 URL에서 계산한다"로 고정한다(fetcher 측 책임).
- `scheduler-claim-api` / DECISIONS.md §3은 `Dequeue(queueType QueueType) (url string, err error)` 시그니처를 고정하며, claim 결과에 `snapshot_key`가 동반되지 않는다. 본 design에서 `Dequeue`를 언급할 때는 이 완전 시그니처를 기본 계약으로 지칭한다.

본 design은 이 "빈 칸"을 좁은 합의 한 장으로 메워, 두 change의 구현이 만났을 때 consumer-fetcher 어댑터가 형태·errorKind·의존성 방향에서 어긋나지 않도록 한다. `Dequeue`나 `Fetcher.Fetch`의 기존 시그니처는 건드리지 않는다.

## Goals / Non-Goals

**Goals:**
- Consumer가 호출하는 snapshot-first fetch 진입점의 반환 형태(html/errorKind/err)를 행위 계약으로 고정한다.
- Consumer가 관측하는 `errorKind` 집합을 fetch 단 4종(`http_4xx`, `http_5xx`, `network`, `timeout`)으로 좁혀, 이후 단계(`parse`, `pin_create`)의 errorKind와 선형적으로 구분되도록 한다.
- Consumer 모듈에 ObjectStorage/HTTP 클라이언트 의존이 유출되지 않도록 의존성 방향을 고정한다.
- Consumer가 반환된 `errorKind`를 재분류 없이 그대로 `RecordHarvestError`에 전달하도록 경계 조건을 고정한다(단, `SetStatus`/`RecordHarvestError` 이중 호출 규약 자체는 `harvester-scheduler-consumer`가 정의하며 본 change는 변경하지 않음).

**Non-Goals:**
- `CompositeFetcher`/`ObjectStorageFetcher`/`HTTPFetcher`의 내부 합성 규칙 → `harvester-snapshot-first-fetch`.
- 스냅샷 키 포맷/해시/빌더 → `pioneer-snapshot-storage`.
- `scheduler.Dequeue`의 반환 형태 변경 → 본 change는 변경하지 않는다.
- `SetStatus`/`RecordHarvestError` 이중 호출 규약 전체 → `harvester-scheduler-consumer`.
- Consumer 루프 전체 단계 순서와 `parse`/`pin_create` errorKind 결정 → `harvester-scheduler-consumer`.
- HTTP fallback 결과의 ObjectStorage 재저장 → `pioneer-snapshot-storage`.
- Fetch retry/backoff 수식 → `scheduler-retry-backoff`.

## Decisions

### Decision 1: fetch 단계 진입점을 별도 레이어로 둔다 (consumer는 `Fetcher.Fetch`를 직접 호출하지 않는다)

**선택**: `harvester-snapshot-first-fetch`가 정의한 `Fetcher.Fetch(url) ([]byte, error)`는 저수준 인터페이스로 유지하되, consumer는 그 위에 얹힌 "snapshot-first fetch 진입점"만 호출한다. 이 진입점의 역할은 (a) `Fetcher.Fetch`를 내부 호출하고, (b) 반환된 `error`를 fetch 단 4종 `errorKind`로 매핑하는 어댑터다. 구현 상의 함수 이름은 `harvester-snapshot-first-fetch` 구현 모듈이 결정하며, `harvester-scheduler-consumer`의 design.md 의사코드는 이를 `fetchSnapshotOrLive`로 예시한다. 본 spec은 이름을 고정하지 않는다.

**근거**:
- `Fetcher.Fetch`를 consumer가 직접 호출하면 error 분류가 consumer로 유출되어 의존성 경계가 무너진다.
- 이름을 spec에 고정하면 리팩터링 저항성이 낮아진다. "snapshot-first 진입점 하나로만 접근"이라는 행위 계약이면 충분.

**대안**: consumer가 `Fetcher.Fetch`를 직접 호출하고 반환 error를 자체 분류. → HTTP 상태 검사/에러 타입 검사 로직이 consumer에 들어간다. 기각.

### Decision 2: 진입점 반환은 `(html []byte, errorKind string, err error)` 3-tuple 의미론

**선택**: 행위 계약은 다음과 같다.

- 성공: `html`은 비어 있지 않은 원본 HTML 바이트열, `err = nil`, `errorKind`는 consumer가 사용하지 않는 값(그 구체적 값은 행위 계약의 관심사가 아님).
- 실패: `html = nil`, `err != nil`, `errorKind ∈ {"http_4xx", "http_5xx", "network", "timeout"}`.

입력은 `(ctx, url)` 두 가지이며, consumer는 `scheduler.Dequeue(QueueHarvester)`가 반환한 URL을 ctx와 함께 그대로 전달한다(기존 `Dequeue` 계약 유지). 스냅샷 키는 진입점 내부에서 `pioneer-snapshot-storage` 공용 빌더로 계산된다(`harvester-snapshot-first-fetch` Decision 5).

**근거**:
- `Dequeue`의 기존 계약(Context §7)을 유지하려면 진입점 입력은 `(ctx, url)`이 자연스럽다. snapshot_key를 claim에 추가하려면 `scheduler-claim-api` 변경이 필요하므로 본 change 범위를 벗어난다.
- `errorKind`를 반환 형태에 포함시키면 consumer는 fetch 단 실패를 그대로 `RecordHarvestError`에 전달 가능. 추가 추론이 필요 없다.

**대안**: 구조체 리턴(`FetchResult{...}`). → 허용 가능한 구현 선택지이며, 본 spec은 3-tuple 의미론(값 3개)만 고정하고 실제 Go 타입(named struct vs multiple return)은 구현 단에 위임한다.

**ctx 전파와 타임아웃 관리**: prerequisite `harvester-snapshot-first-fetch`의 `Fetcher.Fetch(url) ([]byte, error)`는 `ctx` 인자를 받지 않는다. 따라서 snapshot-first 진입점 레이어가 `ctx` 취소 전파와 자체 deadline을 관리한다(예: `Fetcher.Fetch` 호출을 고루틴+`select`로 감싸고 `ctx.Done()`에서 탈출, 또는 내부 HTTP 클라이언트/ObjectStorage SDK의 자체 타임아웃 설정). "timeout" `errorKind`는 이 진입점 레이어의 deadline/ctx 체크로 결정되며, Fetcher 시그니처 변경을 요구하지 않는다.

### Decision 3: `errorKind`는 fetch 단 4종으로 한정한다

**선택**: 진입점이 반환하는 `errorKind`는 `"http_4xx" | "http_5xx" | "network" | "timeout"` 4종으로만 제한한다. `"parse"`, `"pin_create"`는 fetch 이후 단계에서 consumer가 직접 결정하여 `RecordHarvestError`에 전달한다(이미 `harvester-scheduler-consumer` spec에 정의됨).

**근거**:
- fetch 진입점은 HTML 바이트를 돌려주는 책임이며, 이후 파싱/Pin 생성 결과를 알 수 없다.
- 단계별 책임과 kind의 1:1 대응을 유지하면 로그/메트릭에서 어느 단계 실패인지 선형적으로 분간 가능하다.

**대안**: `"snapshot_error"` 같은 별도 kind를 추가해 ObjectStorage 실패를 구분. → `harvester-snapshot-first-fetch` Decision 2("조회 실패는 모두 miss")와 충돌. 기각.

### Decision 4: 스냅샷 내부 실패 종류는 consumer의 `errorKind`에 노출되지 않는다 (prerequisite 재확인)

**선택**: ObjectStorage 조회 실패(키 없음/만료/네트워크/권한/내부)는 `harvester-snapshot-first-fetch`의 "단일 miss" 규약대로 진입점 내부에서 흡수되며, consumer가 받는 `errorKind`에는 반영되지 않는다. 실패 종류는 관측성(로그/메트릭)에만 남는다. 본 change는 이 행위를 새로 정의하지 않고, consumer-fetcher 경계에서 다시 한 번 확인하는 형태로 기록한다.

**근거**:
- prerequisite의 동작을 그대로 가져오지 않으면 consumer가 "snapshot miss도 errorKind로 받을 수 있는가?"를 잘못 가정할 여지가 있다. 명시적 기록이 구현 혼선을 방지한다.
- `harvester-snapshot-first-fetch`는 `bot` capability에 속하고 본 change는 `harvester` capability에 속한다. 두 capability 파일이 분리되어 있어 단일 capability 내부에서 관찰 계약을 완결 짓기 위해 경계 계약을 `harvester` 쪽에 한 번 더 고정한다.

**대안**: 본 design에서 언급하지 않고 prerequisite에만 위임. → consumer 관점의 단일 경계 계약이 파편화된다. 기각.

### Decision 5: Consumer 모듈은 ObjectStorage/HTTP 클라이언트를 직접 의존하지 않는다

**선택**: Consumer 패키지의 import 그래프에 ObjectStorage SDK(S3/MinIO 등)이나 `net/http` 클라이언트 인스턴스 생성부가 등장해서는 안 된다. 이 의존은 `harvester-snapshot-first-fetch` 구현 모듈 안에만 존재한다.

**근거**: 책임 경계의 정적 검증. 리뷰어/CI는 import 그래프를 스캔해 위반을 조기 발견할 수 있다.

**대안**: 인터페이스 의존만 허용. → 의도는 같지만 "구현체 생성부가 consumer 모듈에 없음"을 추가 제약으로 박으면 실수 방지가 쉬워진다.

### Decision 6: `errorKind` 재분류 금지는 fetch 실패 경로에만 적용된다

**선택**: Consumer는 진입점이 반환한 `errorKind`를 **fetch 실패 경로에 한해** 그대로 `RecordHarvestError`에 전달한다. parse/pin_create 경로에서는 consumer가 자체 kind를 결정해 `RecordHarvestError`를 호출하며, 이는 "재분류"가 아니라 서로 다른 실패 단계다. 또한 `SetStatus("harvest_failed", nil)` + `RecordHarvestError(...)` 이중 호출 규약 자체는 `harvester-scheduler-consumer`가 정의하며 본 change는 변경하지 않는다.

**근거**: 리뷰 과정에서 "consumer가 errorKind를 재분류하지 않는다"가 전 실패 경로로 과잉 적용될 수 있음을 식별. 범위 한정 문구를 명시해 오인을 방지한다.

**대안**: 재분류 금지 범위를 spec에 명시하지 않고 암묵적으로 둠. → prerequisite와 읽기 충돌. 기각.

## Risks / Trade-offs

- **[Risk] 4종 `errorKind`만으로는 운영 분석이 부족할 수 있다.** → Mitigation: `harvester-snapshot-first-fetch`가 ObjectStorage 실패 종류를 로그/메트릭으로 별도 기록(관측성 계층). consumer가 받는 kind는 alerting/backoff용으로 단순 유지.
- **[Trade-off] `Dequeue` 반환에 `snapshot_key`를 포함시키지 않음** → fetcher가 URL에서 키를 재계산한다. 해시 한 번의 연산 비용은 무시 가능하며, `scheduler-claim-api`를 건드리지 않아 본 change의 scope가 "integration 계약만 정의"로 유지된다.
- **[Risk] 진입점 어댑터가 `Fetcher.Fetch`의 `error`를 4종으로 매핑할 때 분류 오류 가능성.** → Mitigation: 분류 로직을 헬퍼 함수로 집중화하고, 4종 외 값을 반환하지 않음을 구현 단의 단위 테스트로 커버. 본 spec은 production behavior만 규정한다.
- **[Risk] 본 change가 순수 spec delta이므로 구현 change가 따라오지 않으면 계약만 떠 있게 됨.** → Mitigation: tasks.md의 수용 기준에 "향후 구현 change가 본 spec의 네 Requirement를 수용 기준으로 가져갈 수 있는 형태" 확인 항목을 둔다.

## Migration Plan

1. (선결) `harvester-scheduler-consumer` 적용 → `harvester` capability와 consumer 루프 정의 존재.
2. (선결) `harvester-snapshot-first-fetch` 적용 → `Fetcher`/`CompositeFetcher` 의미론과 스냅샷 키 내부 계산 규약 존재.
3. 본 change 적용: `harvester` capability에 진입점 반환 규약 requirement 묶음 추가.
4. 후속 구현 change: snapshot-first fetch 진입점을 `harvester-snapshot-first-fetch` 구현 모듈에 export하고, consumer 루프가 이를 호출하도록 연결. 본 spec의 네 Requirement를 수용 기준으로 가져간다.
5. **Rollback**: spec 레벨 rollback은 본 change의 ADDED delta 제거만 수행하면 된다. 구현은 여전히 prerequisite 두 change의 계약만으로 동작 가능하지만, consumer의 errorKind 분류 경계가 헐거워진다.
