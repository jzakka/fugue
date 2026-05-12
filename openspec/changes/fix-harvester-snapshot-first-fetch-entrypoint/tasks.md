## 1. 진입점 신설

- [x] 1.1 `apps/api/internal/bot/snapshot_first_fetch.go` 신규 작성. `SnapshotFirstFetch` 인터페이스(`Fetch(ctx context.Context, url string) (html []byte, kind scheduler.ErrorKind, err error)`)와 구현체 `SnapshotFirstFetcher` 정의. 내부에 `objectStorage Fetcher`, `http Fetcher` 두 의존성을 보유. design.md Decision 1.
- [x] 1.2 같은 파일에 `HTTPStatusError` 구조화 에러 타입을 정의(필드 `Code int`). 진입점 내부에서 `errors.As`로 status code를 분류한다. design.md Decision 7.
- [x] 1.3 `apps/api/internal/bot/fetcher.go`의 `HTTPFetcher.Fetch`가 HTTP status 실패 시 `HTTPStatusError`를 반환하도록 보강한다. 기존 `Error() string` 메시지 포맷("... status code %d ...")은 유지하여 외부 호환을 깨지 않는다. 변경 전 `grep -rn "HTTPFetcher\|status code" apps/api/`로 다른 호출처(Pioneer 등)에서 에러 메시지 문자열 매칭이나 `errors.Is/As`로 검사 중인 곳이 있는지 조사하여 영향이 있으면 별도 보강을 한다. design.md Decision 7.
- [x] 1.4 진입점의 `Fetch` 구현: (a) `ctx.Err()` 우선 검사, (b) `objectStorage.Fetch` 시도, (c) ObjectStorage 어떤 실패도 HTTP fallback로 흡수(ctx 취소는 예외), (d) HTTP fallback 결과의 4종 kind 매핑. design.md Decision 2, 3.
- [x] 1.5 kind 매핑 함수: `ctx`/`net.Error.Timeout()` → `timeout`, `HTTPStatusError` 4xx/5xx → `http_4xx`/`http_5xx`, 그 외 transport 에러 → `network`. 4종 외 값을 반환하지 않는다. design.md Decision 2.

## 2. Consumer 경계 정리

- [x] 2.1 `apps/api/internal/bot/harvester_consumer.go`의 `fetcher` 필드 타입을 `Fetcher`에서 `SnapshotFirstFetch`로 교체. 생성자 `NewHarvesterConsumer`의 시그니처도 동일 자리에서 교체.
- [x] 2.2 같은 파일에서 `body, fetchErr := h.fetcher.Fetch(rawURL)` 호출을 `body, fetchKind, fetchErr := h.fetcher.Fetch(ctx, rawURL)`로 교체하고, 실패 분기에서 `classifyHarvestFetchError`로 kind를 재계산하던 로직 대신 `kind := fetchKind`를 사용한다. design.md Decision 4.
- [x] 2.3 `classifyHarvestFetchError`, `harvestStatusCodeFromErr`, `harvestHTTPStatusErrorPattern` 3개 심볼을 삭제한다. 삭제 전 `grep -n "regexp\." apps/api/internal/bot/harvester_consumer.go`로 `regexp` import의 다른 참조가 없음을 확인한 뒤에만 `regexp` import도 함께 정리한다(다른 참조가 남아 있으면 import는 유지). spec "consumer는 fetch 실패 경로에서 errorKind를 재분류하지 않는다" Scenario를 enforce. design.md Decision 4.
- [x] 2.4 변경 라인 근처에 "spec: harvester `Consumer는 snapshot-first 진입점만 경유하여 fetch를 수행한다` Requirement, fetch errorKind 재분류 금지 Scenario를 enforce" 한 줄 주석을 남긴다.

## 3. Production wiring

- [x] 3.1 `apps/api/cmd/bot/harvester_consumer_builder.go`(또는 main wiring 파일)에서 기존 `CompositeFetcher` 인스턴스를 `bot.NewSnapshotFirstFetcher(objectStorageFetcher, httpFetcher)`로 감싸 `NewHarvesterConsumer`에 전달한다. design.md Decision 5.
- [x] 3.2 production wiring 변경 후 `cd apps/api && go build ./...`가 통과하는지 확인한다.

## 4. 단위/회귀 테스트

- [x] 4.1 `apps/api/internal/bot/snapshot_first_fetch_test.go` 신규 작성. 다음 케이스를 각각 별도 `func Test...` 또는 테이블 드리븐으로 추가:
  - HTTP 4xx mock → `kind == scheduler.ErrorHTTP4xx`, `err != nil`, `html == nil`
  - HTTP 5xx mock → `kind == scheduler.ErrorHTTP5xx`
  - transport 에러(`errors.New("dial: ...")`) → `kind == scheduler.ErrorNetwork`
  - `net.Error.Timeout() == true` mock → `kind == scheduler.ErrorTimeout`
  - 사전에 cancel된 ctx → `kind == scheduler.ErrorTimeout`, HTTP fallback 미호출
  - ObjectStorage 성공 → `kind == scheduler.ErrorKind("")` 또는 zero, `err == nil`, `html` non-empty
  - ObjectStorage 실패(어떤 에러든) + HTTP 성공 → 결과는 성공(`err == nil`, kind zero). consumer는 ObjectStorage 실패 종류를 관측하지 않는다.
  - ObjectStorage 실패 + HTTP 5xx → `kind == scheduler.ErrorHTTP5xx`. design.md Decision 6.
- [x] 4.2 `apps/api/internal/bot/harvester_consumer_test.go`의 기존 `classifyHarvestFetchError` 직접 호출 테스트를 삭제하거나, 새 진입점 mock을 사용하는 회귀 테스트로 교체한다. consumer가 `RecordHarvestError`에 전달하는 kind가 진입점 반환값과 동일함을 검증한다 ("consumer는 fetch 실패 경로에서 errorKind를 재분류하지 않는다" Scenario enforce).
- [x] 4.3 consumer 회귀: 진입점이 4종 kind 각각을 반환했을 때 `scheduler.RecordHarvestError(url, kind)`가 정확히 그 kind로 호출됨을 검증.

## 5. 검증

- [x] 5.1 `cd apps/api && go build ./...` 통과.
- [x] 5.2 `cd apps/api && go test ./...` 통과 (전체 테스트).
- [x] 5.3 `grep -n "\.fetcher\.Fetch\|classifyHarvestFetchError\|harvestStatusCodeFromErr\|harvestHTTPStatusErrorPattern" apps/api/internal/bot/harvester_consumer.go apps/api/internal/bot/harvester_consumer_test.go` 결과가 0건임을 확인(`harvester_consumer.go`/그 테스트에서 저수준 Fetcher 호출과 재분류 심볼이 모두 사라졌는지). 신규 진입점 파일(`snapshot_first_fetch.go`) 및 그 테스트(`snapshot_first_fetch_test.go`) 내부 호출은 본 검증 대상이 아니다.
- [x] 5.4 `openspec validate fix-harvester-snapshot-first-fetch-entrypoint --strict` 통과.
