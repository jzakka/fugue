## Why

DESIGN.md L94 'Skeleton loading: 카드 자리에 shimmer 효과'는 카드 자리 한정 명시이고 페이지네이션 로딩 표시는 직접 명세 외 영역이다. 다만 `apps/web/`의 페이지네이션 로딩 표시 4곳을 측정한 결과 spinner 패턴 3곳(FeedContainer L212 auto-load · PinsGrid L135 auto-load · SearchClient L411 manual-click 더보기 버튼 내 spinner) vs 텍스트 패턴 1곳(`apps/web/src/app/boards/[id]/LoadMorePins.tsx:46` "불러오는 중..." 텍스트)로 75% 패턴 외톨이가 발견됐다. 같은 manual-click 카테고리 안에서 SearchClient는 spinner, LoadMorePins는 텍스트로 비대칭. 코드베이스 측정 가능한 패턴 outlier 정렬은 archive/2026-05-15-cancel-button-disabled-opacity(16곳 중 13곳 적용·3곳 잔여 disabled:opacity-50 정렬) 처리 사례와 동일한 논리.

## What Changes

- `apps/web/src/app/boards/[id]/LoadMorePins.tsx:46`의 버튼 자식 표현 `loading ? "불러오는 중..." : "더보기"`를 SearchClient L411 동일 spinner 패턴으로 정렬: loading=true 시 `<div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto" />`, loading=false 시 `"더보기"` 텍스트.
- 사용처 코드 외 추가 변경 없음. 버튼 className/padding/disabled 동작 미변경.

## Capabilities

### New Capabilities
- `design-tokens`: `apps/web/`의 코드 사용 패턴을 DESIGN.md SSoT 또는 코드베이스 측정 패턴에 맞춰 정렬하는 디자인 시스템 정합 capability. 본 변경은 페이지네이션 로딩 표시의 코드베이스 측정 75% 패턴(spinner)에 outlier 1건(LoadMorePins 텍스트)을 정렬한다.

### Modified Capabilities
(없음)

## Impact

- **변경 파일**: `apps/web/src/app/boards/[id]/LoadMorePins.tsx` 단일 파일 1줄(JSX 표현식 1개) 교체.
- **사용처 영향**: 보드 상세 페이지의 "더보기" 버튼 클릭 후 로딩 in-flight 동안 표시가 "불러오는 중..." 텍스트 → spinner(가로 20px·세로 20px·accent border)로 교체. 코드베이스 다른 3곳 페이지네이션 로딩(FeedContainer auto-load·PinsGrid auto-load·SearchClient 검색 더보기)과 시각 일관성 회복.
- **의존성**: 추가 없음. 사용된 Tailwind 유틸리티(`animate-spin`·`border-accent`·`border-t-transparent`·`rounded-full`)는 모두 기존 토큰 사용 중.
- **API·DB**: 영향 없음.
- **롤백**: 단일 커밋 git revert.
