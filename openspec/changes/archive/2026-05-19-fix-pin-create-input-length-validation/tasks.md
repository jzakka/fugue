# Tasks: POST /api/pins 입력 길이 검증

## 1. 코드 변경

- [ ] 1.1 `apps/api/internal/pin/handler.go` import 블록에 `"unicode/utf8"` 추가.
- [ ] 1.2 L96 직후(title 빈값 검증 다음)에 `if utf8.RuneCountInString(title) > 200 { writeError(w, http.StatusBadRequest, "제목은 200자 이내여야 합니다"); return }` 추가.
- [ ] 1.3 L288 내부(description trim 직후, `if d != ""` 블록 안)에 `if utf8.RuneCountInString(d) > 500 { writeError(w, http.StatusBadRequest, "설명은 500자 이내여야 합니다"); return }` 추가.
- [ ] 1.4 L293 내부(url trim 직후, `if u != ""` 블록 안)에 `if utf8.RuneCountInString(u) > 1000 { writeError(w, http.StatusBadRequest, "URL은 1000자 이내여야 합니다"); return }` 추가.
- [ ] 1.5 L298 내부(og_image trim 직후, `if o != ""` 블록 안)에 `if utf8.RuneCountInString(o) > 1000 { writeError(w, http.StatusBadRequest, "og_image URL은 1000자 이내여야 합니다"); return }` 추가.

## 2. 테스트

- [ ] 2.1 `apps/api/internal/pin/handler_test.go`에 title 길이 검증 subtest 추가:
  - `TestCreate_RejectsTitleOverRuneCap`: ASCII 201자 입력 → 400 + 메시지 일치 + CreatePin 미호출
  - `TestCreate_RejectsTitleOverRuneCapMultibyte`: 한국어 201 rune 입력 → 400 + 메시지 일치 + CreatePin 미호출
  - `TestCreate_AcceptsTitleAtRuneCap`: ASCII 200자 입력 → 201 + CreatePin 호출
  - `TestCreate_AcceptsTitleAtRuneCapMultibyte`: 한국어 200 rune 입력 → 201 + CreatePin 호출

## 3. Spec 변경

- [ ] 3.1 `openspec/changes/fix-pin-create-input-length-validation/specs/pin/spec.md`에 ADDED Requirement 1건 (4 Scenario 포함).

## 4. 사전 조건 검증 (gates)

- [ ] 4.1 `cd apps/api && go vet ./...` 통과.
- [ ] 4.2 `cd apps/api && go build ./...` 통과.
- [ ] 4.3 `cd apps/api && go test ./...` 전체 통과 (새 테스트 포함).

## 5. 실 환경 QA

- [ ] 5.1 `docker-compose up -d`로 api+postgres+redis 기동.
- [ ] 5.2 `cd apps/api && go run cmd/server/main.go`.
- [ ] 5.3 인증 세션 획득.
- [ ] 5.4 4개 필드 각각에 cap+1 ASCII 입력 → 400 + 정확한 메시지.
- [ ] 5.5 4개 필드 각각에 cap+1 한국어 입력 → 400 + 정확한 메시지(특히 title 한국어 201 rune 케이스).
- [ ] 5.6 4개 필드 각각에 정확히 cap 길이 입력 → 201 + DB 저장.
- [ ] 5.7 회귀: 일반 입력으로 핀 생성 → 201.
- [ ] 5.8 회귀(인접 엔드포인트): `GET /api/pins/{id}` 조회 → 200.

## 6. 머지·아카이브·로그

- [ ] 6.1 커밋 & PR (PR 본문에 evidence·인용·QA 결과 그대로).
- [ ] 6.2 CI green 확인 후 squash-merge + remote branch 삭제.
- [ ] 6.3 OpenSpec change를 `archive/2026-05-19-fix-pin-create-input-length-validation/`로 이동.
- [ ] 6.4 backlog 항목 `in_progress` → `done` + resolution 노트.
- [ ] 6.5 `.fugue/decision-log.md`에 사이클 마감 엔트리 1건 추가.
