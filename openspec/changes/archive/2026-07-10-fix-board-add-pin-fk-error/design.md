# fix-board-add-pin-fk-error — Design

## Context

`POST /api/boards/{id}/pins` 핸들러(`apps/api/internal/boards/handler.go`의 `AddPin`)는 보드 존재·소유권만 확인한 뒤 곧바로 `AddPinToBoard`(board_pins insert)를 실행한다. `board_pins.pin_id`에는 FK 제약이 있어 존재하지 않는 핀이면 DB가 FK violation을 반환하는데, 현재는 모든 insert 오류가 일괄적으로 500 "핀을 추가할 수 없습니다"로 매핑된다.

## Decisions

### D1. 핀 존재 사전 확인 (pre-check) 방식 채택

insert 전에 `GetPin` 쿼리로 핀 존재를 확인하고, `sql.ErrNoRows`면 404 "핀을 찾을 수 없습니다"를 반환한다.

- 대안: pq FK violation 코드(23503) 검사 후 404 변환. 단일 쿼리로 끝나지만, 코드베이스에 드라이버 오류 코드 검사 패턴이 전무하고(`pq.Error` 사용처 0곳) 핸들러 계층에 드라이버 의존을 새로 들이게 된다.
- 채택 근거: 기존 핸들러들이 이미 `GetBoard` + `sql.ErrNoRows` 사전 확인 패턴을 쓰고 있어 일관적이다. 추가 쿼리 1회는 이 엔드포인트의 트래픽 특성상 무시 가능하다.
- TOCTOU: 사전 확인과 insert 사이에 핀이 삭제되는 극단적 경합은 기존과 동일하게 500으로 남는다. 이 창은 실질적으로 무시 가능하며, 별도 처리하지 않는다.

### D2. 404 선택 (400이 아니라)

요청 본문이 가리키는 리소스(핀)가 존재하지 않는 경우이므로, 형식 오류(400)가 아닌 찾을 수 없음(404)으로 응답한다. 보드가 없거나 타인 보드일 때 404를 주는 기존 관례와 정렬된다.

### D3. 응답 메시지

"핀을 찾을 수 없습니다" — 보드 404("보드를 찾을 수 없습니다")와 동일한 어조의 한국어 메시지.

### D4. 테스트 주입 지점: 최소 querier 인터페이스

boards `Handler`는 현재 `*sql.DB`를 직접 들고 각 핸들러에서 `db.New(h.database)`를 호출한다. 코드베이스에 sqlmock·테스트 DB 관례가 없으므로, pin 패키지 선례(`Handler.q PinQuerier`)를 따라 boards `Handler`에 querier 인터페이스 필드를 도입한다.

- `boardsQuerier` 인터페이스: 핸들러들이 사용하는 sqlc 메서드 집합(GetBoard, CreateBoard, UpdateBoard, DeleteBoard, ListBoards 계열, CountBoardPins, ListBoardPinImages, ListBoardPins, GetTagsForPins, AddPinToBoard, RemovePinFromBoard, GetPin, CreateInteraction 등)만 선언. `*db.Queries`가 자연 충족한다.
- `NewHandler(database *sql.DB)` 시그니처는 불변 — 내부에서 `db.New(database)`를 q에 저장.
- 각 핸들러의 `q := db.New(h.database)`를 `q := h.q`로 치환한다. 트랜잭션 사용처가 없어 안전하다.
- 단, `hydratePinTags`는 두 번째 파라미터로 구체 타입 `*db.Queries`를 받고 있어 단순 치환만으로는 컴파일되지 않는다. 이 파라미터 타입을 `boardsQuerier`로 변경하고, 인터페이스에 `GetTagsForPins`를 포함해 호출부(`GetByID`)가 인터페이스 값을 그대로 넘길 수 있게 한다.

## Test Strategy

- 핸들러 단위 테스트(같은 패키지, mock querier 직접 주입): 존재하지 않는 핀 ID로 AddPin 요청 시 404 "핀을 찾을 수 없습니다"를 검증한다. 핀이 존재하는 정상 경로(201)·중복(409)·타인 보드(404) 회귀 시나리오도 mock으로 함께 검증한다.
- 기존 검증 경로 테스트(body cap 등, `boards_test` 외부 패키지)는 변경 없이 통과해야 한다.
