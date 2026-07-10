# fix-board-add-pin-fk-error — Tasks

## 1. 테스트 주입 지점 마련

- [x] 1.1 `apps/api/internal/boards/handler.go`에 `boardsQuerier` 인터페이스를 정의하고 `Handler`가 이를 보유하도록 변경 (`NewHandler(database *sql.DB)` 시그니처 불변, 내부에서 `db.New(database)` 저장)
- [x] 1.2 각 핸들러의 `q := db.New(h.database)`를 `q := h.q`로 치환하고, `hydratePinTags`의 `*db.Queries` 파라미터를 `boardsQuerier`로 변경(인터페이스에 `GetTagsForPins` 포함) 후 `go build ./...` 통과 확인

## 2. AddPin 오류 매핑 수정

- [x] 2.1 `AddPin`에서 보드 소유권 확인 직후 `GetPin`으로 핀 존재를 확인하고, `sql.ErrNoRows`면 404 "핀을 찾을 수 없습니다" 반환 (그 외 조회 오류는 500 유지)

## 3. 테스트

- [x] 3.1 mock querier로 존재하지 않는 핀 추가 시 404와 메시지를 검증하는 테스트 추가
- [x] 3.2 핀이 존재하는 정상 추가(201)·중복 추가(409)·타인 보드(404) 회귀 테스트 추가
- [x] 3.3 `apps/api`에서 `go build ./...` 및 `go test ./...` 통과 확인

## 4. 검증

- [x] 4.1 로컬 환경(docker-compose Postgres) 기동이 가능하면 실제 요청으로 존재하지 않는 핀 추가 → 404, 정상 추가 → 201, 중복 → 409를 QA 확인 (불가 시 사유 명시)
