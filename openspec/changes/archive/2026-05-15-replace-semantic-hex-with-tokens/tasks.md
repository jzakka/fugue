## 1. 토큰 클래스로 교체

- [x] 1.1 `apps/web/src/components/board/AddToBoardButton.tsx` L237: `bg-[#34C759]/10 border border-[#34C759]/30 text-[#34C759]` → `bg-success/10 border border-success/30 text-success`.
- [x] 1.2 같은 파일 L238: `bg-[#FF3B30]/10 border border-[#FF3B30]/30 text-[#FF3B30]` → `bg-error/10 border border-error/30 text-error`.

## 2. 검증

- [x] 2.1 `grep #34C759|#FF3B30 apps/web/src --include='*.tsx' --include='*.ts'` 결과 0건 확인.
- [x] 2.2 AddToBoardButton.tsx L237-238에 `bg-success/10`, `border-success/30`, `text-success`, `bg-error/10`, `border-error/30`, `text-error` 모두 적용 확인.
- [~] 2.3 `npm run build` — worktree에 node_modules 없어 미실행. 변경이 className 문자열 6 token 교체로 빌드 로직 영향 없음. main 워크트리에서 후속 실행 권장.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "Semantic 토큰화" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-semantic-hex-literal` status를 `done`으로 변경.
