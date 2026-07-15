# Tasks: loadmore-button-fill-vocab

## 1. 구현

- [x] 1.1 `apps/web/src/app/boards/[id]/LoadMorePins.tsx:45` 더보기 버튼 className 정합화 — `py-2.5` → `py-3`, `py-3` 뒤에 `bg-surface` 삽입(SearchClient:422 와 어휘 순서 미러링). 그 외 클래스·JSX 구조·핸들러 미변경.

## 2. 검증

- [x] 2.1 `cd apps/web && npm run lint && npx tsc --noEmit && npm test -- --run` 통과 (lint 0 errors·19 pre-existing warnings / tsc 0 / vitest 47 passed)
- [x] 2.2 실 브라우저 QA — qa_plan 수행: (1) /boards/[id] 더보기 버튼이 bg-surface+py-3 로 렌더되어 /search 더보기와 computed style(배경색·padding) 일치, (2) 클릭 시 loadMore·spinner·disabled·hasMore=false unmount 유지, (3) hover/focus-visible 회귀 0, (4) 인접 회귀: SearchClient 더보기·FeedContainer noscript 앵커 무변경, light 테마 대조
