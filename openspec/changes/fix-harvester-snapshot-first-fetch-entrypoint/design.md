## Context

`openspec/specs/harvester/spec.md` L563-657은 Harvester consumer가 fetch 단계를 호출할 때 ① 단일 진입점만 경유, ② 입력은 `(ctx, url)`, ③ 반환은 `(html, errorKind, err)` 3-tuple, ④ 실패 시 `errorKind`는 4종 한정, ⑤ ObjectStorage 내부 실패는 진입점 내부에서 흡수라는 5가지 행위를 SHALL로 규정합니다. 현재 코드는 다음 위반을 가집니다.

- `apps/api/internal/bot/fetcher.go:40-42` — `Fetcher.Fetch(url) ([]byte, error)` 2-tuple. ctx도 errorKind도 없습니다.
- `apps/api/internal/bot/harvester_consumer.go:313` — consumer가 `h.fetcher.Fetch(rawURL)`로 저수준 Fetcher를 직접 호출.
- `apps/api/internal/bot/harvester_consumer.go:321,490-527` — consumer가 `classifyHarvestFetchError`로 에러 메시지를 regex 파싱하여 status code를 재추출, errorKind를 재분류.

archive change `2026-04-23-harvester-snapshot-first-fetchconsumer/proposal.md`가 위 5개 Requirement를 spec에 추가했으나, 후속 구현 change는 부재합니다. 본 change가 그 후속 구현입니다.

## Goals / Non-Goals

**Goals:**
- Consumer 측 fetch 경계에 한 개의 진입점 심볼만 남깁니다. `Fetcher.Fetch` 직접 호출과 consumer 재분류 로직을 제거합니다.
- 진입점은 `(ctx, url) → (html, errorKind, err)` 의미론을 구현하며 실패 시 4종 errorKind(`http_4xx`/`http_5xx`/`network`/`timeout`) 중 하나만 반환합니다.
- ObjectStorage 조회 실패 종류를 consumer에 노출하지 않습니다. ctx 취소/deadline은 `timeout`으로 귀결합니다.
- production wiring(`apps/api/cmd/bot/*`)에서 `CompositeFetcher`(ObjectStorage-first → HTTP fallback)를 진입점 안으로 옮깁니다.

**Non-Goals:**
- 기존 `Fetcher`/`CompositeFetcher`/`ObjectStorageFetcher`/`HTTPFetcher` 타입 자체를 삭제하지 않습니다. 이들은 진입점 내부 구현 부품으로 유지됩니다.
- scheduler 계약(`Dequeue`/`SetStatus`/`RecordHarvestError`)이나 그 호출 규약은 변경하지 않습니다.
- parse/pin_create 경로의 `RecordHarvestError(url, "parse"/"pin_create")` 호출은 본 change의 4종 한정 대상이 아닙니다. spec L637-639에서 명시적으로 예외 처리됨.
- harvester 외 도메인의 fetch 경로(Pioneer 등)는 손대지 않습니다.

## Decisions

### Decision 1: 진입점은 인터페이스로 추상화, 구현체는 `SnapshotFirstFetcher` 구조체

새 인터페이스 `SnapshotFirstFetch`를 `apps/api/internal/bot/snapshot_first_fetch.go`에 둡니다.

```
type SnapshotFirstFetch interface {
    Fetch(ctx context.Context, url string) (html []byte, kind scheduler.ErrorKind, err error)
}
```

구체 타입 `SnapshotFirstFetcher`가 이를 구현하며 내부에 `objectStorage` Fetcher와 `http` Fetcher를 들고 있습니다. Consumer는 인터페이스만 주입받으므로 ObjectStorage/HTTP 클라이언트 구체 타입을 알지 못합니다. 이 결정으로 spec L571-573("consumer 모듈에 ObjectStorage/HTTP 클라이언트 의존 부재")를 정적으로 만족합니다.

**대안**: 함수 타입 `type SnapshotFirstFetchFn func(ctx, url) ([]byte, scheduler.ErrorKind, error)`를 직접 주입. 단순하지만 mock/테스트 편의성을 위해 인터페이스를 채택.

### Decision 2: errorKind 매핑 규칙

진입점 내부 매핑 표:

| 상황 | 반환 kind |
|---|---|
| ctx.Err() == DeadlineExceeded 또는 Canceled (snapshot/http 어느 단계든) | `timeout` |
| `net.Error.Timeout() == true` | `timeout` |
| HTTP 응답 status 4xx | `http_4xx` |
| HTTP 응답 status 5xx | `http_5xx` |
| 그 외 (DNS, connect, TLS, EOF 등 transport-level 실패) | `network` |
| ObjectStorage 조회 실패 (어떤 종류든) | consumer에 노출되지 않음. HTTP fallback 결과만 노출. |

성공 경로에서는 `kind`를 `scheduler.ErrorKind("")` 또는 동일한 빈 zero value로 두며, consumer는 성공 분기에서 kind를 사용하지 않습니다(spec L599-601).

`net.Error.Timeout()`은 `errors.As`로 unwrap 후 검사합니다. HTTP 응답이 도착했으나 status가 4xx/5xx면 transport는 성공이므로 `http_4xx`/`http_5xx`가 우선이고, 그 이전 단계 실패만 `network`가 됩니다.

### Decision 3: snapshot 내부 실패는 무조건 HTTP fallback으로 흡수

`objectStorage.Fetch`가 어떤 에러를 반환하든(키 없음, 만료, 권한, 5xx, 내부 에러) HTTP fallback을 실행합니다. 단 ctx 취소/deadline은 흡수하지 않고 즉시 `timeout`으로 종료합니다(spec L655-657). 이는 `CompositeFetcher.Fetch`가 이미 가진 "any error → HTTP fallback" 동작을 재사용하되, ctx 검사 단계만 새로 추가합니다.

ObjectStorage 단계 실패 종류는 로그(`slog`)로만 구분합니다(spec L645 "로그로만 구분된다" 재확인).

### Decision 4: consumer 변경 범위 최소화

- `harvester_consumer.go`의 `fetcher` 필드 타입을 `Fetcher`에서 `SnapshotFirstFetch`로 교체.
- 생성자 `NewHarvesterConsumer`의 시그니처에서 같은 자리를 교체. 호출처(`apps/api/cmd/bot/harvester_consumer_builder.go`, 기존 테스트 헬퍼)도 함께 갱신.
- `body, fetchErr := h.fetcher.Fetch(rawURL)` →
  `body, fetchKind, fetchErr := h.fetcher.Fetch(ctx, rawURL)`
- 실패 분기에서 `kind := classifyHarvestFetchError(fetchErr)` 제거. 대신 `kind := fetchKind`. `RecordHarvestError(rawURL, kind)` 호출은 그대로.
- `classifyHarvestFetchError`, `harvestStatusCodeFromErr`, `harvestHTTPStatusErrorPattern` 3개 심볼 삭제. import `regexp`도 더 이상 필요 없으면 삭제.

### Decision 5: production wiring

`apps/api/cmd/bot/harvester_consumer_builder.go`(또는 main wiring)에서 기존에 만들던 `CompositeFetcher`를 `bot.NewSnapshotFirstFetcher(objectStorageFetcher, httpFetcher)`로 감싸 consumer에 전달합니다. `CompositeFetcher` 자체는 다른 호출처가 있는지 확인 후, 없으면 단계적으로 제거 가능하지만 본 change에서는 wiring만 교체합니다.

### Decision 6: 테스트 전략

- 단위 테스트 `snapshot_first_fetch_test.go` 추가:
  - HTTP 4xx → `http_4xx` (mock http fetcher가 `&httpStatusError{code: 404}` 같은 표지 에러 반환)
  - HTTP 5xx → `http_5xx`
  - DNS/connect/TLS 실패 → `network`
  - 자체 timeout (`net.Error.Timeout() == true`) → `timeout`
  - ctx.Cancel → `timeout`
  - ObjectStorage miss → HTTP fallback 성공 (kind = zero, err = nil)
  - ObjectStorage error (any) → HTTP fallback 수행
  - ObjectStorage 동안 ctx.Cancel → `timeout` (HTTP fallback 시도 안 함)
- consumer 회귀 테스트는 새 진입점 mock으로 갱신. 기존 `classifyHarvestFetchError` 직접 테스트는 삭제.
- 정적 회귀: `grep -n 'h.fetcher.Fetch'` 결과 0건 확인은 코드 리뷰 시 수행. (테스트 자동화 대상 아님)

### Decision 7: HTTP 상태코드 전달 메커니즘

기존 `HTTPFetcher`는 4xx/5xx를 `fmt.Errorf("... status code %d", code)`로 알리는 free-form 에러 메시지를 반환합니다. 새 진입점에서는 regex 파싱을 다시 도입하지 않기 위해, 진입점 내부의 HTTP fetch 단계가 `*HTTPStatusError{Code int}` 같은 구조화 에러 타입을 반환하도록 합니다. 이 타입은 진입점 패키지 내부에서만 사용되며 consumer로는 노출되지 않습니다(consumer는 kind만 봅니다).

구체 위치:
- `apps/api/internal/bot/snapshot_first_fetch.go`(또는 인접 파일)에 `HTTPStatusError` 정의.
- `HTTPFetcher.Fetch`가 status 실패 시 이 타입을 반환하도록 수정 (기존 메시지 포맷도 유지하여 외부 호환을 보장하되, `errors.As`로 분류 가능하게 함).

이로써 spec L633-635("HTTP 상태코드/에러 타입을 다시 검사하여 kind를 재결정하는 로직을 포함해서는 안 된다")는 진입점 내부에서 단 한 번만 분류하고, consumer는 그 결과를 패스스루하는 형태로 만족합니다.

## Risks / Trade-offs

- **Risk**: 기존 `Fetcher`/`CompositeFetcher`를 호출하는 다른 코드 경로가 남아있을 가능성 → Mitigation: `grep -rn '\.fetcher\.Fetch\|CompositeFetcher{' apps/api`로 호출처 전수 조사 후 한꺼번에 교체. 본 change 범위는 harvester만이므로 Pioneer 경로는 손대지 않음.
- **Risk**: `HTTPFetcher` 에러 타입 변경이 다른 테스트의 메시지 매칭을 깰 가능성 → Mitigation: 에러 메시지 포맷은 유지(Error() string은 동일), `errors.As`로 새 타입을 꺼낼 수 있도록만 보강.
- **Trade-off**: 진입점 내부에 작은 인터페이스 두 개(`objectStorage`, `http`) 의존이 그대로 남으므로 unit test 시 mock 두 개를 준비해야 합니다. 단일 구체 함수 호출보다 약간 verbose하지만 4종 kind를 결정짓는 분기 표면이 작아 비용이 작습니다.
- **Trade-off**: ObjectStorage 단계 실패 종류를 consumer가 관측하지 못하므로 운영 시 ObjectStorage 헬스는 진입점 내부 로그에서만 추적 가능합니다. spec L645가 이를 명시적으로 요구하므로 수용.
