# Tasks: create-board-cancel-busy-gating

## 1. 구현

- [x] 1.1 MyPageClient 새 보드 폼 취소 버튼에 `disabled={creating}` + className 끝 `disabled:opacity-50` 추가
- [x] 1.2 AddToBoardButton 새 보드 폼 취소 버튼에 `disabled={creating}` + className 끝 `disabled:opacity-50` 추가
- [x] 1.3 두 컴포넌트의 실패 경로에서 setCreating(false) 복귀 확인 (영구 잠금 없음)

## 2. 검증

- [x] 2.1 lint/typecheck/test 통과
- [x] 2.2 실 브라우저 QA: (1) /mypage 새 보드 폼 — POST 지연 유도 후 creating 중 취소 disabled+디밍, (2) 핀 상세 보드에 추가 모달 새 보드 폼 동일, (3) 유휴 취소 정상(폼 닫힘·입력 초기화), (4) 생성 성공/실패 경로 회귀 없음·콘솔 에러 0
