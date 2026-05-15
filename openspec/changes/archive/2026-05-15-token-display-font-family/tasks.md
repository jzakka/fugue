## 1. 토큰 정의

- [x] 1.1 `apps/web/src/app/globals.css`의 `@theme inline` 블록(L40-56) 마지막에 `--font-display: 'General Sans', sans-serif;` 한 줄을 추가한다. Tailwind v4가 자동으로 `font-display` 유틸리티를 생성한다.

## 2. inline style → className 치환 (8개 파일)

- [x] 2.1 `apps/web/src/app/pins/[id]/page.tsx:138-141` `<h1>`의 inline style 제거 + className 끝에 `font-display` 추가.
- [x] 2.2 `apps/web/src/app/pins/[id]/page.tsx:256-259` `<h2>`의 inline style 제거 + className 끝에 `font-display` 추가.
- [x] 2.3 `apps/web/src/app/pin/new/PinCreateForm.tsx:314-317` `<h1>`의 inline style 제거 + className 끝에 `font-display` 추가.
- [x] 2.4 `apps/web/src/app/search/SearchClient.tsx:232-235` `<h1>`의 inline style 제거 + className 끝에 `font-display` 추가.
- [x] 2.5 `apps/web/src/app/boards/[id]/page.tsx:56-59` `<h1>`의 inline style 제거 + className 끝에 `font-display` 추가.
- [x] 2.6 `apps/web/src/components/board/BoardGrid.tsx:14-17` `<h2>`의 inline style 제거 + className 끝에 `font-display` 추가.
- [x] 2.7 `apps/web/src/components/board/AddToBoardButton.tsx:207-210` `<h2>`의 inline style 제거 + className 끝에 `font-display` 추가.
- [x] 2.8 `apps/web/src/components/profile/MyPageClient.tsx:67-70` `<h2>`의 inline style 제거 + className 끝에 `font-display` 추가.

## 3. 검증

- [x] 3.1 grep으로 `fontFamily.*General Sans` 패턴이 `apps/web/src` 아래에 0건 남음을 확인.
- [x] 3.2 변경된 파일이 globals.css + 위 8개 파일 = 9개임을 git diff로 확인. `apps/web/` 밖 변경 없음.

## 4. 사후 기록

- [x] 4.1 `.fugue/decision-log.md`에 "Display 폰트 패밀리를 @theme inline 토큰화" 항목 1~3줄 추가.
- [x] 4.2 `.fugue/backlog-design.yaml`에서 `design-20260515-font-family-token-missing` 항목 status를 `done`으로 변경 + note 추가.
- [x] 4.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-token-display-font-family/`로 이동.
