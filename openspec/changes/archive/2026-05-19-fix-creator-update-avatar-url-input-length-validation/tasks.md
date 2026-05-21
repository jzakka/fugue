# Tasks

## 1. handler 구현

- [ ] 1.1 `apps/api/internal/creator/handler.go` `UpdateMe` 핸들러 L189-193 `if req.AvatarURL != nil` 분기 안의 `else`(비어있지 않은 값) 가지 안, L192 할당 직전에 `utf8.RuneCountInString(*req.AvatarURL) > 500 → 400 "아바타 URL은 500자 이내여야 합니다"` 블록 추가.
- [ ] 1.2 `unicode/utf8` import 확인(L10에 이미 존재 — 추가 불필요).
- [ ] 1.3 빈 문자열 `""`은 cap 검증을 건너뛰고 기존 clear 동작 유지(L189-190 분기) — 별도 작업 없이 자연 보존.

## 2. 단위 테스트

- [ ] 2.1 `apps/api/internal/creator/handler_test.go`에 신규 subtest 5개 추가(기존 `mockQuerier`/`sampleCreator`/`withCreatorID` 재사용).
- [ ] 2.2 `TestUpdateMe_RejectsAvatarURLOverRuneCap` — ASCII 501 → 400 + UpdateCreator 미호출 확인.
- [ ] 2.3 `TestUpdateMe_RejectsAvatarURLOverRuneCapMultibyte` — 한국어 501 rune → 400 + UpdateCreator 미호출 확인.
- [ ] 2.4 `TestUpdateMe_AcceptsAvatarURLAtRuneCap` — ASCII 500 → 200 + `lastUpdate.AvatarUrl.String` 길이 500 확인.
- [ ] 2.5 `TestUpdateMe_AcceptsAvatarURLEmptyAsClear` — `""` → 200 + `lastUpdate.AvatarUrl.Valid == false`.
- [ ] 2.6 `TestUpdateMe_AcceptsAvatarURLOmitted` — `avatar_url` 누락 → 200 + `lastUpdate.AvatarUrl == current.AvatarUrl`(기존 값 보존).

## 3. 자체 리뷰

- [ ] 3.1 변경 범위가 `apps/api/internal/creator/`로 한정.
- [ ] 3.2 `utf8.RuneCountInString` 사용이 같은 핸들러 L182 nickname 패턴과 동일.
- [ ] 3.3 에러 메시지 형식이 cycle 8/9 패턴과 정렬.
- [ ] 3.4 `decision-log.md` 위반 사항 없음 — cycle 8/9의 동일 area 마지막 후보.

## 4. 게이트

- [ ] 4.1 `cd apps/api && go vet ./...` 통과.
- [ ] 4.2 `cd apps/api && go build ./...` 통과.
- [ ] 4.3 `cd apps/api && go test ./...` 통과.

## 5. 실 환경 QA

- [ ] 5.1 docker-compose up + API 기동 + JWT 발급.
- [ ] 5.2 PUT `/api/creators/me` avatar_url=`A*501` → 400 "아바타 URL은 500자 이내여야 합니다".
- [ ] 5.3 PUT avatar_url=`가*501` → 400 (멀티바이트).
- [ ] 5.4 PUT avatar_url=`A*500` boundary → 200 + 응답 length=500.
- [ ] 5.5 PUT avatar_url=`""` (clear) → 200 + 응답 avatar_url=null.
- [ ] 5.6 PUT avatar_url 누락(nickname만) → 200 + 기존 avatar_url 보존.
- [ ] 5.7 회귀(adjacent): nickname=`A*51` → 400 "닉네임은 50자를 초과할 수 없습니다" 그대로.
- [ ] 5.8 회귀(인접 엔드포인트): GET `/api/creators/me` → 200.
- [ ] 5.9 DB 확인: `SELECT length(avatar_url) FROM creators WHERE avatar_url IS NOT NULL...` 모든 row ≤ 500.

## 6. 머지 후

- [ ] 6.1 커밋: `fix(creator handler): reject avatar_url inputs exceeding creators VARCHAR(500) cap with 400`.
- [ ] 6.2 PR 본문에 evidence·QA 결과 첨부.
- [ ] 6.3 CI green → squash merge.
- [ ] 6.4 워크트리 stale `main` 충돌 시 parent에서 `git pull origin main` + `git push origin --delete loop-system/<branch>`.
- [ ] 6.5 archive로 이동, backlog `done` + resolution, decision-log 1~3줄.
