## 1. CSS 룰 추가

- [x] 1.1 `apps/web/src/app/globals.css`에 `.font-mono { font-variant-numeric: tabular-nums; }` 룰을 `@theme inline` 블록 끝( `}` 닫는 줄 다음) 빈 줄 다음에 추가. 본문 `body` 룰보다는 위, `@theme inline` 닫힘 직후에 위치.

## 2. 검증

- [x] 2.1 grep으로 `font-variant-numeric|tabular-nums`가 globals.css에 1건 존재 확인.
- [x] 2.2 `apps/web/` 밖 변경 없음을 git diff로 확인.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "font-mono tabular-nums 활성화" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-mono-tabular-nums-missing` 항목 status를 `done`으로 변경 + note 추가.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-font-mono-tabular-nums/`로 이동.
