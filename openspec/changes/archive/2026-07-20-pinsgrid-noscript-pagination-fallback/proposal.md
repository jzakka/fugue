# Proposal: PinsGrid noscript 페이지네이션 폴백 정렬

## Why

IntersectionObserver 무한스크롤 표면 전수 2곳 중 FeedContainer(피드)만 `<noscript>` '다음 페이지' 링크 폴백을 보유한다(FeedContainer.tsx:224-235, 서버 app/page.tsx:65 offset searchParam 지원). 동형 아키타입인 PinsGrid(크리에이터 프로필 /creators/[id])는 폴백이 없고 서버 페이지도 searchParams를 지원하지 않아, JS 비활성 환경에서 프로필 핀 초기 배치(20개) 이후 콘텐츠에 접근할 수단이 없다. 동일 아키타입 실갈림은 일관성 정렬 대상이다(img-onerror-degrade 부분 적용 정렬 선례 동형).

## What Changes

- `apps/web/src/app/creators/[id]/page.tsx`: `searchParams`(offset)를 받아 서버 fetchPins에 전달하고 `initialOffset`을 PinsGrid에 내린다 (app/page.tsx:65-68 동형).
- `apps/web/src/components/profile/PinsGrid.tsx`: `initialOffset` prop 추가(offsetRef 초기값 = initialOffset + initialPins.length, FeedContainer:42 동형) + 렌더 말미에 FeedContainer:224-235 동형 `<noscript>` '다음 페이지' 링크 블록 추가. 링크는 offset만 전달(필터 media_type은 클라이언트 상태 전용 — JS 비활성 시 사용 불가이므로 무필터 뷰만 지원, 보수 원칙 d).

## Scope

- 변경 범위: `apps/web/` 내 2파일. API·백엔드 무변경.
- 사용자 영향: JS 활성 사용자에게는 시각·동작 변화 없음(noscript 블록 비렌더·offset 미지정 시 기존 경로와 동일). JS 비활성 사용자는 프로필에서 '다음 페이지' 링크로 페이지네이션 가능.
- 롤백: 두 파일의 diff revert만으로 완전 복원.

## Capabilities

- Modified: `profile` — 크리에이터 프로필 핀 그리드에 스크립트 비활성 페이지네이션 폴백 추가.

## QA Plan

1. `/creators/[id]?offset=N` 서버 렌더가 다음 배치 핀을 표시한다.
2. JS 비활성(CDP `Emulation.setScriptExecutionDisabled`) 상태에서 `/creators/[id]` 하단에 '다음 페이지' 링크가 렌더되고 href가 `?offset=<초기 배치 수>`로 정합하다.
3. JS 활성 시 noscript 블록 비가시, 무한스크롤·필터 회귀 없음(피드 noscript 회귀 대조 포함).
4. 콘솔 에러 0.
