# Tasks

## 1. handler 구현

- [ ] 1.1 `apps/api/internal/boards/handler.go` `Create` 핸들러 L121 `if req.Description != nil` 분기 내부, L122 할당 직전에 `utf8.RuneCountInString(*req.Description) > 500 → 400 "보드 설명은 500자 이내여야 합니다"` 블록 추가.
- [ ] 1.2 같은 파일 `Update` 핸들러 L293 `if req.Description != nil` 분기 내부, L294 할당 직전에 동일 블록 추가.
- [ ] 1.3 `unicode/utf8` import 확인(L11에 이미 존재 — 추가 불필요).

## 2. 단위 테스트

- [ ] 2.1 `apps/api/internal/boards/handler_test.go` 신규 파일(또는 기존 `handler_optional_auth_test.go`에 신규 함수 추가) 생성, package `boards_test`.
- [ ] 2.2 `TestCreate_RejectsDescriptionOverRuneCap` — description ASCII 501자 → 400 + 메시지.
- [ ] 2.3 `TestCreate_RejectsDescriptionOverRuneCapMultibyte` — description 한국어 501 rune → 400.
- [ ] 2.4 `TestCreate_AcceptsDescriptionAtRuneCap` — description ASCII 500자 → DB nil 환경에서 검증 통과, 이후 db 호출 시 panic 또는 nil deref(검증 자체는 트리거 안 됨을 입증).
- [ ] 2.5 Update 경로는 GetBoard 선행 DB 의존으로 단위 테스트에서 제외(real-env QA로 검증). design.md D7 참조.

## 3. 자체 리뷰

- [ ] 3.1 변경 범위가 `apps/api/internal/boards/`로 한정되어 있는지.
- [ ] 3.2 `unicode/utf8` 사용이 같은 핸들러 L110 패턴과 동일한지.
- [ ] 3.3 에러 메시지 형식이 L111 `"보드 이름은 100자 이내여야 합니다"`와 정렬되는지.
- [ ] 3.4 `decision-log.md` 위반 사항 없음 — cycle 8 pin 핸들러 정합성 작업의 동일 area 확장.

## 4. 게이트

- [ ] 4.1 `cd apps/api && go vet ./...` 통과.
- [ ] 4.2 `cd apps/api && go build ./...` 통과.
- [ ] 4.3 `cd apps/api && go test ./...` 통과.

## 5. 실 환경 QA

- [ ] 5.1 docker-compose up + API 기동 + JWT 발급.
- [ ] 5.2 POST `/api/boards` description=`A*501` → 400 "보드 설명은 500자 이내여야 합니다".
- [ ] 5.3 POST `/api/boards` description=`가*501` → 400 (멀티바이트).
- [ ] 5.4 POST `/api/boards` description=`A*500` boundary → 201 정상.
- [ ] 5.5 POST `/api/boards` description 생략 → 201 정상.
- [ ] 5.6 PUT `/api/boards/{id}` description=`B*501` → 400.
- [ ] 5.7 PUT `/api/boards/{id}` description=`나*501` → 400 (멀티바이트).
- [ ] 5.8 PUT `/api/boards/{id}` description=`B*500` boundary → 200.
- [ ] 5.9 PUT description 생략 → 기존 description 보존(L292 merge).
- [ ] 5.10 회귀: GET `/api/boards/{id}` → 200.
- [ ] 5.11 회귀(adjacent field): name=`A*101` → 400 "보드 이름은 100자 이내여야 합니다" 그대로.
- [ ] 5.12 DB 확인: `SELECT length(description) FROM boards ...` 모든 row ≤ 500.

## 6. 머지 후

- [ ] 6.1 커밋: `fix(boards handler): reject description inputs exceeding boards VARCHAR(500) cap with 400`.
- [ ] 6.2 PR 본문에 evidence·QA 결과 첨부.
- [ ] 6.3 CI green → squash merge.
- [ ] 6.4 워크트리 stale `main` 충돌 시 parent에서 `git pull origin main` + `git push origin --delete loop-system/<branch>`.
- [ ] 6.5 archive로 이동, backlog `done` + resolution, decision-log 1~3줄.
