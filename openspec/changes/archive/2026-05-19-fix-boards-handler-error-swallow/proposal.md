## Why

`apps/api/internal/boards/handler.go`의 `Update`(L319-320)와 `ListByCreator`(L398-399)가 `CountBoardPins`/`ListBoardPinImages` DB 조회 에러를 swallow한다. DB 일시 실패 시 응답 contract의 `pin_count: 0` + `cover_images: null`이 200 응답에 그대로 들어가 클라이언트가 잘못된 값을 진실로 취급한다.

같은 파일의 `GetByID`(L173-185)는 동일한 두 쿼리에 대해 명시적으로 `log.Printf` + `writeError(w, http.StatusInternalServerError, "보드 정보를 불러올 수 없습니다")`로 처리한다. 즉 본 결함은 spec text 위반이라기보다 **같은 파일·같은 쿼리에 대한 에러 처리 baseline 불일치**다. baseline은 GetByID의 명시적 500 처리이며, 본 change는 Update와 ListByCreator를 그 baseline에 맞춘다.

## What Changes

- `boards.Update`가 `pin_count`/`cover_images` 조회에 실패하면 200이 아니라 500을 반환한다(`GetByID`와 동일).
- `boards.ListByCreator`가 보드 목록 안의 한 보드에 대한 `pin_count`/`cover_images` 조회에 실패하면 200 부분 응답이 아니라 500을 반환한다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities

- `board`: 기존 Requirement `보드를 수정한다`와 `유저의 보드 목록을 조회한다`에 보조 Scenario 각 1개를 추가해, 응답에 포함되는 `pin_count`/`cover_images` 값의 정확성을 명시한다. 기존 Scenario들의 본문은 변경하지 않는다.

## Impact

- 영향 코드: `apps/api/internal/boards/handler.go`의 `Update`/`ListByCreator` 두 함수.
- 운영 지표: DB 일시 실패 시 200 + 잘못된 값 → 500. 정상 경로는 변하지 않는다.
- 의존성, DB 스키마, 마이그레이션 없음.
