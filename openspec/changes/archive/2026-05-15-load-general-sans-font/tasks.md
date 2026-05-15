## 1. 폰트 로딩 라인 추가

- [x] 1.1 `apps/web/src/app/layout.tsx`의 `<head>` 내부에 `<link rel="stylesheet" href="https://api.fontshare.com/v2/css?f[]=general-sans@500,700&display=swap" />`을 추가한다. 기존 Pretendard / Geist Mono `<link>` 사이 또는 직후에 배치해 같은 그룹임을 보인다.

## 2. 검증

- [~] 2.1 `cd apps/web && npm run build` 통과 — worktree에 node_modules 없어 미실행. 변경이 정적 `<link>` 1줄로 TSX 타입체크/번들 로직과 무관해 회귀 위험 사실상 없음. main 워크트리에서 다음 실행 시 확인.
- [x] 2.2 layout.tsx의 `<link>` 3개 확인 (Pretendard L22-25 / Geist Mono L17-20 / General Sans L26-29).
- [x] 2.3 inline `fontFamily: "'General Sans', sans-serif"` 사용처 8건(7개 파일) 그대로 유지 확인.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "General Sans 로딩 추가" 항목을 1~3줄로 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-general-sans-not-loaded` 항목 status를 `done`으로 변경.
