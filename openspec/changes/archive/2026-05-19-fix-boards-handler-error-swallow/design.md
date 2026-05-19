## Context

`apps/api/internal/boards/handler.go`의 4개 핸들러(`Create`/`GetByID`/`Update`/`ListByCreator`) 중 두 곳(`Create`는 생성 직후라 항상 `pin_count: 0`/`cover_images: nil`이 의미상 정합)을 제외한 `GetByID`/`Update`/`ListByCreator` 세 곳이 동일한 두 sqlc 쿼리(`CountBoardPins`, `ListBoardPinImages`)를 호출해 응답 페이로드를 채운다.

- `GetByID` L173-185: 두 쿼리 모두 `if err != nil` 검사 + `log.Printf` + `writeError(500, "보드 정보를 불러올 수 없습니다")` 명시 처리.
- `Update` L319-320: `pinCount, _ := q.CountBoardPins(...)`, `images, _ := q.ListBoardPinImages(...)` — 에러 swallow. zero value(`pinCount=0`, `images=nil`)가 200 응답에 그대로 들어감.
- `ListByCreator` L398-399: 보드 루프 안에서 동일하게 swallow. 한 보드의 일시 실패가 그 보드만 잘못된 값으로 표시.

`BoardResponse`는 `pin_count` int64와 `cover_images` []string을 응답 contract로 노출한다. 클라이언트는 응답 코드 200을 신뢰의 기준으로 사용하므로, swallow된 zero value가 200과 함께 반환되면 그것을 진실로 취급한다.

본 결함은 spec text의 직접 SHALL 위반이라기보다 **코드 baseline 불일치**다. baseline은 GetByID이며, 동일 쿼리·동일 contract에 대해 비대칭 에러 처리를 두면 운영 가시성·클라이언트 신뢰가 모두 깨진다.

## Goals / Non-Goals

### Goals

- `boards.Update` 응답이 `pin_count`/`cover_images` 조회 실패 시 200을 반환하지 않도록 한다. `GetByID`와 동일한 500 처리로 정렬한다.
- `boards.ListByCreator` 응답이 보드 목록 안에서 한 보드의 보조 쿼리 실패가 발생할 때 200 부분 응답을 반환하지 않도록 한다. 동일하게 500으로 정렬한다.
- 정상 경로(`CountBoardPins`/`ListBoardPinImages` 둘 다 성공)의 응답은 변하지 않는다.

### Non-Goals

- spec text의 응답 정확성 SHALL을 새 Requirement로 신설하지 않는다. 본 결함은 코드 baseline 일관성으로 충분히 정당화되며, 기존 Requirement에 보조 Scenario를 추가하는 것이 최소 변경이다.
- `Create` 핸들러는 다루지 않는다(생성 직후 zero value가 의미상 정합).
- `CountBoardPins`/`ListBoardPinImages` 쿼리 자체의 강건성(재시도·circuit breaker)은 본 change 범위 밖.
- 클라이언트(`apps/web/`)의 에러 처리 분기는 본 change 범위 밖.

## Decisions

### Decision 1: 옵션 A(500 정렬) 채택

세 옵션을 검토했다.

- **(A) Update/ListByCreator의 두 쿼리에 GetByID와 동일한 500 처리** ← 채택.
  - 장점: 같은 파일 안의 baseline에 즉시 정합. 잘못된 zero value가 응답에 들어가지 않는다. 클라이언트가 200/500 분기로 정확/실패를 구분할 수 있다.
  - 단점: DB 일시 실패가 200 → 500으로 가시화되어 클라이언트 retry 부담이 약간 증가. 그러나 200 + 잘못된 값보다는 운영 가시성·신뢰 측면에서 우월.

- (B) log만 추가하고 200 + zero value는 유지.
  - 장점: 변경 폭 최소, 클라이언트 영향 0.
  - 단점: 응답 contract의 잘못된 값은 그대로 나간다. 결함의 본질(잘못된 값 노출)을 닫지 않고 운영 가시성만 회복.

- (C) ListByCreator는 best-effort partial 응답(에러 발생 보드만 `pin_count: null`로 명시) + spec text 변경.
  - 장점: 보드 목록 가용성이 한 보드의 부분 실패에 영향받지 않음.
  - 단점: spec text의 응답 contract 변경(`pin_count: null` 허용)이 필요. `BoardResponse.PinCount`를 `*int64`로 바꾸는 등 폭이 커지고 클라이언트 분기도 추가됨. 최소 해결이 아님.

(A)가 baseline 정합 + 최소 변경 + 클라이언트 신뢰 보존을 모두 만족.

### Decision 2: Update와 ListByCreator의 에러 메시지

- `Update`: GetByID와 동일하게 "보드 정보를 불러올 수 없습니다"로 통일. (Update 자체는 성공했으나 응답 조립이 실패한 케이스이므로 "수정에 실패" 메시지보다 정확.)
- `ListByCreator`: "보드 목록을 불러올 수 없습니다" (기존 L391-393 동일 핸들러의 다른 에러 메시지와 정합).

### Decision 3: ListByCreator 루프의 fail-fast vs partial

루프 안에서 한 보드의 보조 쿼리가 실패하면 즉시 500을 반환한다(fail-fast). 이미 처리한 보드들의 데이터를 부분 응답으로 내보내지 않는다. 이유:

- 응답 contract가 `boards: [...]` 단일 배열이라 부분 데이터를 어떤 형태로 알릴 surface가 없다.
- 클라이언트가 부분 응답을 알아채는 표준이 없으므로 정확/오류 두 상태 사이의 모호한 중간 상태를 만들지 않는 것이 안전.

## Risks / Trade-offs

- **R1**: DB 일시 실패가 ListByCreator에서 보드 한 건만 영향을 줘도 전체 응답이 500이 된다.
  - 완화: GetByID와 baseline 정렬이 최우선. partial 응답은 spec text + DTO 변경이 필요한 별도 결함으로 분리.
- **R2**: 기존 클라이언트가 ListByCreator의 부분 실패를 묵시적으로 무시(0/null 표시)하고 있었다면 500 응답이 새 에러 surface로 노출됨.
  - 완화: 0/null이 표시된 보드는 이미 사용자에게 잘못된 데이터였으므로 500이 더 정직한 신호다.

## Migration Plan

1. `boards.Update` L319-320의 `pinCount, _ := ...`, `images, _ := ...`를 GetByID L173-185와 동일한 if-err 체크 + 500 응답으로 교체.
2. `boards.ListByCreator` L398-399의 두 쿼리도 동일 처리. 루프 안에서 에러 발생 시 fail-fast로 500 반환.
3. `boards/handler_test.go`(또는 기존 test 파일)에 회귀 방지 단위 테스트 2개 추가:
   - `TestUpdate_ReturnsInternalServerErrorWhenCountBoardPinsFails`
   - `TestListByCreator_ReturnsInternalServerErrorWhenBoardImagesFails`
4. `openspec/changes/fix-boards-handler-error-swallow/specs/board/spec.md`에 보조 Scenario 2개 ADDED (기존 Requirement 본문 변경 없음).
5. archive 시점에 spec 머지.

## Open Questions

없음.
