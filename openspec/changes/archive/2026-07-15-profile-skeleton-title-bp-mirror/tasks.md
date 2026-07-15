# Tasks: profile-skeleton-title-bp-mirror

## 1. 구현

- [x] 1.1 `apps/web/src/components/profile/ProfileSkeleton.tsx:9` 제목 바 클래스를 `h-8` → `h-8 sm:h-9`로 변경

## 2. 검증

- [x] 2.1 `cd apps/web && npm run lint && npm run typecheck && npm test -- --run` 통과 (typecheck 스크립트 부재 → `npx tsc --noEmit` 대체, lint 0 errors·tsc 통과·vitest 47/47)
- [x] 2.2 실 브라우저 QA: ≥sm(1280px) 스켈레톤 제목 바 computed height 36px, 스왑 후 h1 라인박스 36px 일치·핀 카운트 y-오프셋 불변 (headless Chrome CDP, 핀 상세→크리에이터 클라이언트 내비게이션 MutationObserver+rAF 실측 36=36, pinTop 192↔193)
- [x] 2.3 실 브라우저 QA: <sm(500px) 제목 바 32px 유지 (실측 32=32)
- [x] 2.4 실 브라우저 QA: shimmer 모션·skeleton-shimmer 착색 무변경, 콘솔 에러 0 (animationName shimmer·barBg rgb(30,30,30), 프로필 직접 진입 콘솔 에러 0)
- [x] 2.5 인접 회귀: 피드 CardSkeleton 무변경 확인 (피드 20핀 렌더·콘솔 에러 0·diff 무변경)
