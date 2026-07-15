# Tasks: addtoboard-failure-copy-frame

## 1. 구현

- [x] 1.1 `apps/web/src/components/board/AddToBoardButton.tsx:200` 실패 카피를 "보드 추가에 실패했습니다. 다시 시도해주세요"로 변경

## 2. 검증

- [x] 2.1 `cd apps/web && npm run lint && npm run typecheck && npm test -- --run` 통과
- [x] 2.2 실 브라우저 QA: 보드 추가 실패 유도 → 신규 카피 렌더, 409·성공·보드 생성 실패 카피 무변경, 콘솔 에러 0
