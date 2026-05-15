## 1. 구현

- [x] 1.1 `apps/web/src/components/feed/FeedContainer.tsx:185` div className을 `mb-4 p-4 bg-surface rounded-md border-l-3 border-error text-sm` → `mb-4 p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error`로 교체.

## 2. 검증

- [x] 2.1 grep `border-l-3` apps/web/src 결과 0건 확인.
- [x] 2.2 grep `rounded-md` apps/web/src 결과 1건(NavBar.tsx:15 로고 박스)만 남음 확인.
- [x] 2.3 grep `bg-error/10 border border-error/30` 결과 6건(기존 5건 + FeedContainer 1건)으로 증가 확인.
- [x] 2.4 `apps/web/` 밖 파일 변경 0건 확인(git diff --stat).

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "FeedContainer 에러 박스 표준 패턴 정렬" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-feedcontainer-error-pattern-outlier` 항목 status를 `done`으로 변경 + note 보강.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-align-feedcontainer-error-pattern/`로 이동.
