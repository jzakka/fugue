## 1. 토큰 정의

- [x] 1.1 `apps/web/src/app/globals.css`의 `@theme inline` 블록에 `--font-mono: 'Geist Mono', monospace;` 한 줄을 `--font-display` 다음 줄에 추가한다.

## 2. inline style → className 치환 (23곳)

- [x] 2.1 `apps/web/src/app/pins/[id]/page.tsx:132` `<span>` 미디어 타입 배지 — inline style 제거, className에 `font-mono` 추가.
- [x] 2.2 `apps/web/src/app/pins/[id]/page.tsx:156` 동일 패턴.
- [x] 2.3 `apps/web/src/app/pin/new/PinCreateForm.tsx:340` `<div>` 미디어 타입 hint — 동일 패턴.
- [x] 2.4 `apps/web/src/app/pin/new/PinCreateForm.tsx:393` `<span>` 확장자 배지 — 동일 패턴.
- [x] 2.5 `apps/web/src/app/pin/new/PinCreateForm.tsx:400` 동일.
- [x] 2.6 `apps/web/src/app/pin/new/PinCreateForm.tsx:409` 동일.
- [x] 2.7 `apps/web/src/app/pin/new/PinCreateForm.tsx:507` 도메인/사이트명 — 동일.
- [x] 2.8 `apps/web/src/app/pin/new/PinCreateForm.tsx:536` 태그 칩 — 동일.
- [x] 2.9 `apps/web/src/app/pin/new/PinCreateForm.tsx:597` 태그 칩 — 동일.
- [x] 2.10 `apps/web/src/app/search/SearchClient.tsx:267` 카테고리 탭 — 동일.
- [x] 2.11 `apps/web/src/app/search/SearchClient.tsx:331` 크리에이터 가입일 — 동일.
- [x] 2.12 `apps/web/src/app/search/SearchClient.tsx:388` 메타데이터(객체 분리 표기) — 실제로는 단일 속성(`fontFamily`)만 있어 동일 패턴으로 처리.
- [x] 2.13 `apps/web/src/app/boards/[id]/page.tsx:65` `<span>` 핀 카운트 — 동일.
- [x] 2.14 `apps/web/src/components/pin/VideoTrimModal.tsx:146` 시간 표시 — 동일.
- [x] 2.15 `apps/web/src/components/pin/VideoTrimModal.tsx:207` 전체 시간 — 동일.
- [x] 2.16 `apps/web/src/components/board/BoardGrid.tsx:31` 핀 카운트 — 동일.
- [x] 2.17 `apps/web/src/components/board/AddToBoardButton.tsx:292` 핀 카운트 — 동일.
- [x] 2.18 `apps/web/src/components/profile/MyPageClient.tsx:138` 핀 카운트 — 동일.
- [x] 2.19 `apps/web/src/components/profile/ProfileHeader.tsx:50` 핀 카운트 — 동일.
- [x] 2.20 `apps/web/src/components/feed/PinCard.tsx:182` 태그 칩 — 동일.
- [x] 2.21 `apps/web/src/components/feed/TagFilter.tsx:49` 초기화 버튼 — 동일.
- [x] 2.22 `apps/web/src/components/feed/TagFilter.tsx:65` 태그 칩 — 동일.
- [x] 2.23 `apps/web/src/components/nav/SearchBar.tsx:294` 미디어 타입 배지 — 동일.

## 3. 검증

- [x] 3.1 grep으로 `fontFamily.*Geist Mono` 패턴이 `apps/web/src` 아래에 0건 남음을 확인.
- [x] 3.2 변경된 파일이 globals.css + 12개 소스 = 13개임을 git diff로 확인 (tasks.md의 "13개 파일/14개" 표현은 고유 경로 12개를 23 inline-style 항목과 혼동한 표기 오류였음). `apps/web/` 밖 변경 없음.

## 4. 사후 기록

- [x] 4.1 `.fugue/decision-log.md`에 "Mono 폰트 패밀리를 @theme inline 토큰화" 항목 1~3줄 추가. anti-pattern L15 검토 결과(font-mono 사용처 0건이라 적용 대상 아님) 명시.
- [x] 4.2 `.fugue/backlog-design.yaml`에서 `design-20260515-tags-mono-font-token-missing` 항목 status를 `done`으로 변경 + note 추가.
- [x] 4.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-token-mono-font-family/`로 이동.
