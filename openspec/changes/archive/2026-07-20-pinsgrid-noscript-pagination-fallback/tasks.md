# Tasks

## 1. 서버 페이지 offset 지원

- [x] 1.1 `apps/web/src/app/creators/[id]/page.tsx`: Props에 `searchParams` 추가, offset 해석(parseInt || 0), `fetchPins`에 offset 전달, `PinsGrid`에 `initialOffset` 전달

## 2. PinsGrid noscript 폴백

- [x] 2.1 `apps/web/src/components/profile/PinsGrid.tsx`: `initialOffset` prop 추가(기본 0), `offsetRef = useRef(initialOffset + initialPins.length)`
- [x] 2.2 렌더 말미에 FeedContainer:224-235 동형 `<noscript>` '다음 페이지' 링크 블록 추가(`?offset=N`만 전달)

## 3. 검증

- [x] 3.1 lint(0 errors)·tsc --noEmit(0)·vitest 47 passed 통과
- [x] 3.2 실 브라우저 QA: headless Chrome CDP로 qa_plan 4항목 통과(offset=20 서버렌더 8핀, JS비활성 링크 href 정합·피드 동일 표기, JS활성 noscript 비가시·무한스크롤 20→28, 콘솔 에러 0)
