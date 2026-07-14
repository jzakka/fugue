# Tasks: pinsgrid-filter-active-fill-vocab

## 1. 구현

- [x] 1.1 `apps/web/src/components/profile/PinsGrid.tsx` 필터 버튼 className 정합화
  - 활성 `bg-accent text-white` → `bg-text-primary text-bg`
  - 비활성 `hover:border-accent`·`focus-visible:border-accent` → `hover:border-text-muted`·`focus-visible:border-text-muted`
  - chip 공통 `py-2` → `py-1.5`, `font-medium` 추가
  - 비활성 base `bg-surface border border-border text-text-muted` 및 hover/focus 텍스트 전이는 유지

## 2. 검증

- [x] 2.1 `cd apps/web && npm run lint && npm run typecheck && npm test -- --run`
- [x] 2.2 실 브라우저 QA: 프로필 페이지(/creators/[id]) 미디어타입 필터 활성 칩이 반전 어휘로 렌더 + hover 보더 text-muted + aria-pressed/필터링 동작 유지, 피드 FieldFilter 회귀 무 확인
