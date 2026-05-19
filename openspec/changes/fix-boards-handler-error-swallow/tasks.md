## Tasks

- [ ] 1. `apps/api/internal/boards/handler.go`의 `Update` 핸들러 에러 정렬
  - [ ] 1.1 L319-320의 `pinCount, _ := q.CountBoardPins(...)`, `images, _ := q.ListBoardPinImages(...)`를 두 갈래 if-err 체크로 분리한다.
  - [ ] 1.2 분기 1(`pinCount` 조회 실패): `log.Printf("boards.Update: count pins error: %v", err)` + `writeError(w, http.StatusInternalServerError, "보드 정보를 불러올 수 없습니다")` + return.
  - [ ] 1.3 분기 2(`images` 조회 실패): `log.Printf("boards.Update: list images error: %v", err)` + 동일 500 응답 + return.
  - [ ] 1.4 정상 경로 응답(`writeJSON(w, http.StatusOK, toBoardResponse(updated, pinCount, images))`)은 변경하지 않는다.

- [ ] 2. `apps/api/internal/boards/handler.go`의 `ListByCreator` 핸들러 에러 정렬
  - [ ] 2.1 L398-399의 루프 안 `pinCount, _ := ...`, `images, _ := ...`를 if-err 체크로 분리한다.
  - [ ] 2.2 보드 한 건의 보조 쿼리 실패 시 즉시 `log.Printf("boards.ListByCreator: <kind> error: %v", err)` + 500 응답 + return (fail-fast).
  - [ ] 2.3 루프 정상 경로(`results = append(results, toBoardResponse(b, pinCount, images))`)는 변경하지 않는다.

- [x] 3. (보류) 단위 테스트 추가
  - 사유: `apps/api/internal/boards/handler.go`의 `Handler`가 `*sql.DB`를 직접 받고 핸들러 안에서 매번 `db.New(h.database)`로 querier를 만든다. mock querier를 주입하려면 `BoardQuerier` 인터페이스 + `NewHandlerWithQuerier` surface 도입이 필요한데, 이는 본 결함(에러 처리 baseline 정렬)과 별개 카테고리 변경(의존 역전)이며 변경 폭이 핸들러 본문 패턴 전반에 영향을 준다.
  - 본 사이클은 핸들러 본문 패턴 변경 없이 두 분기의 에러 처리만 정렬한다. 회귀 방지는 step 7 실 환경 QA(docker-compose postgres stop으로 DB 실패를 결정적으로 재현)로 수행한다.
  - 인터페이스 surface 도입은 별도 결함 후보로 다룰 수 있다(본 사이클 범위 밖).

- [ ] 4. `openspec/changes/fix-boards-handler-error-swallow/specs/board/spec.md`에 ADDED Scenario 2개 작성
  - [ ] 4.1 기존 Requirement `보드를 수정한다`에 1개 Scenario를 ADDED ("보드 정보 조회가 실패한 경우").
  - [ ] 4.2 기존 Requirement `유저의 보드 목록을 조회한다`에 1개 Scenario를 ADDED ("보드 목록 내 보조 정보 조회가 실패한 경우").
  - [ ] 4.3 두 Scenario 모두 응답 코드를 명시하지 않고 "거부한다"는 surface 행위 계약만 명시한다.

- [ ] 5. 검증·통합
  - [ ] 5.1 `cd apps/api && go vet ./...` 통과.
  - [ ] 5.2 `cd apps/api && go build ./...` 통과.
  - [ ] 5.3 `cd apps/api && go test ./internal/boards/...` 통과 (신규 테스트 포함).
  - [ ] 5.4 `cd apps/api && go test ./...` 통과 (전체 회귀 없음).
  - [ ] 5.5 실 환경 QA: docker-compose 환경에서 정상 케이스(200 + 정확한 값)와 에러 케이스(docker-compose stop postgres 후 500)를 직접 curl로 확인.
  - [ ] 5.6 archive: capability main spec(`openspec/specs/board/spec.md`)의 두 Requirement에 신규 Scenario 머지. 기존 Scenario들의 본문·순서는 변경하지 않는다.
