# Tasks: board-edit-enter-submit

## 1. 구현

- [x] 1.1 BoardActions.tsx 이름 입력(:74)에 onKeyDown Enter → handleSave 핸들러 추가 (MyPageClient:124 idiom)
- [x] 1.2 BoardActions.tsx 설명 입력(:85)에 동일 핸들러 추가

## 2. 검증

- [x] 2.1 `npm run lint && npx tsc --noEmit && npm test -- --run` 통과
- [x] 2.2 실 브라우저 QA: /boards/<id> 편집 모드에서 이름 입력 Enter → 저장·편집 모드 종료·이름 반영, 설명 입력 Enter → 저장, 빈 이름 Enter → 검증 에러, /mypage 보드 생성 폼 Enter 회귀 없음, 콘솔 에러 0

## 3. 스펙 동기화

- [x] 3.1 specs/board delta를 openspec/specs/board/spec.md 본문에 반영, `openspec validate --specs` 통과
