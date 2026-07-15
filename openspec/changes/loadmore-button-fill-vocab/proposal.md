# Proposal: loadmore-button-fill-vocab

## Why

동일한 역할(수동 "더보기" 추가 로드 컨트롤)의 버튼이 표면 간 다른 시각 어휘로 렌더된다. 검색 결과(SearchClient)와 피드 noscript 폴백(FeedContainer)은 `bg-surface` 채움 + `py-3` pill 로 균일한데, 보드 상세(LoadMorePins)만 채움 없는 `py-2.5` pill 이다. 같은 라벨("더보기")·같은 동작의 컨트롤이 페이지마다 다르게 보여 cross-surface 일관성이 깨진다.

## What Changes

- `apps/web/src/app/boards/[id]/LoadMorePins.tsx` 더보기 버튼 className 을 manual load-more 아키타입의 확립 어휘로 정합화: `py-2.5` → `py-3`, `bg-surface` 추가. 그 외 클래스(hover/focus-visible/disabled/transition 등)와 JSX 구조·동작은 미변경.
- 코드 변경은 이 1개 파일, 1줄 className 뿐이다. SearchClient·FeedContainer 는 canonical 표면이므로 손대지 않는다.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `board`: 보드 상세 페이지의 추가 핀 로드 컨트롤("더보기" 버튼)이 다른 표면(검색 결과)의 동일 역할 컨트롤과 동일한 시각 표현으로 렌더된다는 요구를 명시.

## Impact

- 영향 코드: `apps/web/src/app/boards/[id]/LoadMorePins.tsx:45` (className 1줄)
- 사용자 영향: 보드 상세의 더보기 버튼 배경이 surface 색으로 채워지고 세로 패딩이 2px 커진다(12px). 동작·레이아웃 구조 변화 없음.
- 롤백: className 1줄 revert 로 즉시 복원 가능.
- API/의존성/토큰: 변경 없음.
