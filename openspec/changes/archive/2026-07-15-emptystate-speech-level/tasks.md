# Tasks: emptystate-speech-level

## 1. 구현

- [x] 1.1 `apps/web/src/components/feed/FeedContainer.tsx:165` EmptyState 메시지를 "이 분야의 작품이 아직 없어요" → "이 분야의 작품이 아직 없습니다"로 변경
- [x] 1.2 `apps/web/src/components/feed/__tests__/FeedContainer.test.tsx:87` 단언 문자열을 새 문구로 동기 갱신

## 2. 검증

- [x] 2.1 `npx tsc --noEmit` 통과
- [x] 2.2 `npx vitest run` 통과
- [x] 2.3 실 브라우저 QA: 피드 분야 필터 빈 결과 → "이 분야의 작품이 아직 없습니다" 렌더, 마스코트/레이아웃 무변경, 프로필·보드 빈 상태 문구 회귀 없음, 콘솔 에러 0
