# Tasks: HarvestPipeline title rune-safe truncate

## 1. 코드 변경

- [ ] 1.1 `apps/api/internal/bot/harvester_consumer.go`의 `processOne` 함수에서 `doc.BodyText = truncateRunes(doc.BodyText, 500)` 바로 다음 줄에 `doc.Title = truncateRunes(doc.Title, 200)` 추가.
- [ ] 1.2 같은 파일 상단의 comment(L380 부근)를 "Rune-safe truncation to fit pins.description (500 runes) and pins.title (200 runes)"로 확장.
- [ ] 1.3 `truncateRunes` 함수의 doc-comment(L490-492)를 "Used to fit PinDocument.BodyText / PinDocument.Title into the pins.description / pins.title column rune limits before persistence."로 업데이트.

## 2. 문서 변경

- [ ] 2.1 `apps/api/internal/bot/pin_document.go:36-38`의 PinDocument doc-comment에 "Title is raw text BEFORE the title-length cut; the Harvester is responsible for the rune-safe truncation when persisting." 한 줄을 BodyText 줄과 대칭으로 추가.

## 3. 테스트

- [ ] 3.1 `apps/api/internal/bot/harvester_consumer_title_truncate_test.go` 신규 파일 생성. `truncateRunes`를 직접 검증하는 4 케이스 추가:
  - ASCII 201자 → 200자 출력 확인
  - 한국어 201 rune → 200 rune 출력 확인 + 마지막 글자가 한국어 글자 단위로 깨끗하게 잘렸는지
  - 100자 정상 → 무손실
  - 빈 문자열 → 빈 문자열

## 4. Spec 변경

- [ ] 4.1 `openspec/specs/harvester/spec.md` "PinDocument 부가 필드 og_data 저장 정책" 섹션에 ADDED Scenario "title은 pins.title 컬럼에 200 rune 이내로 잘라 저장한다" 1건 추가.

## 5. 사전 조건 검증

- [ ] 5.1 `cd apps/api && go vet ./...` 통과.
- [ ] 5.2 `cd apps/api && go build ./...` 통과.
- [ ] 5.3 `cd apps/api && go test ./...` 전체 통과 (새 테스트 포함).

## 6. 실 환경 QA

- [ ] 6.1 `docker-compose up -d`로 api+postgres 기동.
- [ ] 6.2 호스트에서 201 rune title을 가진 페이지를 serve.
- [ ] 6.3 Pioneer 1회 + Harvester 1회 실행.
- [ ] 6.4 API 로그에 `value too long for type character varying(200)` 부재 확인.
- [ ] 6.5 `psql`로 생성된 pins row의 title 길이 ≤200, 멀티바이트 경계 안전 확인.
- [ ] 6.6 정상 길이 title의 회귀 없음 확인.

## 7. 머지·아카이브·로그

- [ ] 7.1 커밋 & PR (PR 본문에 evidence·인용·QA 결과 그대로).
- [ ] 7.2 CI green 확인 후 squash-merge + remote branch 삭제.
- [ ] 7.3 OpenSpec change를 `archive/2026-05-19-fix-harvest-pipeline-title-truncate/`로 이동.
- [ ] 7.4 backlog 항목 `in_progress` → `done` + resolution 노트.
- [ ] 7.5 `.fugue/decision-log.md`에 사이클 마감 엔트리 1건 추가.
