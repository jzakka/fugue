# Tasks

## 1. 구현

- [x] 1.1 `apps/web/src/components/board/AddToBoardButton.tsx` 보드 추가 실패 fallback 메시지에서 ". 다시 시도해주세요" 접미 제거

## 2. 검증

- [x] 2.1 tsc 통과 확인
- [x] 2.2 vitest 통과 확인 (기존 테스트가 해당 문자열을 참조하면 함께 갱신)
- [x] 2.3 실 브라우저 QA: 보드 추가 실패 강제 → 무접미 메시지, 보드 생성 실패 문형 유지, 정상 플로우 회귀 없음, 콘솔 에러 0
