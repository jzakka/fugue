## 1. 구현

- [x] 1.1 `apps/web/src/app/page.tsx` `<main className="flex-1 pb-12">` 첫 자식으로 `<h1 className="sr-only">작품 피드</h1>` 한 줄 추가.

## 2. 검증

- [x] 2.1 grep `<h1` apps/web/src/app/page.tsx 결과 1건 확인.
- [x] 2.2 grep `sr-only.*작품 피드` apps/web/src/app/page.tsx 결과 1건 확인.
- [x] 2.3 본 사이클 변경은 page.tsx 단일 파일에 한정됨.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "메인 피드 페이지 h1 sr-only 추가" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-home-page-h1-missing` 항목 status를 `done`으로 변경 + note 보강.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-home-page-h1-heading/`로 이동.
