## Why

DESIGN.md L11 "Decoration level: Minimal — 타이포그래피와 여백이 모든 걸 한다." 명세는 UI 장식을 최소화한다는 원칙이다. `apps/web/`의 5곳 가로 스크롤 영역(메인 피드의 미디어 타입 필터·인기 태그 칩, 검색 페이지의 카테고리 탭·인기 태그 칩, 핀 등록 폼의 태그 카테고리 탭)에 `scrollbar-hide` className이 부여되어 작성자가 OS 기본 가로 스크롤바를 숨기려는 의도를 명시했으나, 해당 클래스가 `apps/web/src/app/globals.css`에 정의되지 않았고 Tailwind v4 코어 utilities·PostCSS 플러그인 어느 쪽에도 포함되지 않아 무효 상태다. 결과적으로 5곳에서 OS 기본 가로 스크롤바가 그대로 노출되어 "Minimal" 명세에 어긋난다.

## What Changes

- `apps/web/src/app/globals.css`에 `.scrollbar-hide` CSS 룰 1개 블록 추가 (`-ms-overflow-style: none; scrollbar-width: none; &::-webkit-scrollbar { display: none; }`).
- 추가는 `@theme inline` 블록 밖의 plain CSS rule로, Tailwind 토큰 시스템에 영향을 주지 않는다.
- 사용처 5곳의 className·코드는 미변경.

## Capabilities

### New Capabilities
- `design-tokens`: `apps/web/src/app/globals.css`가 DESIGN.md를 단일 진실 원천(SSoT)으로 매핑하는 디자인 토큰·유틸리티 정의 capability. 본 변경은 가로 스크롤 영역의 스크롤바 가시성을 제어하는 utility class를 추가한다.

### Modified Capabilities
(없음)

## Impact

- **변경 파일**: `apps/web/src/app/globals.css` 단일 파일, 수 줄(2개 CSS 룰 블록) 추가.
- **사용처 영향**: 5곳 가로 스크롤 영역에서 OS 기본 가로 스크롤바가 숨겨진다. 가로 스크롤 자체 동작(터치 스와이프, 휠 가로 스크롤, 키보드 화살표)은 그대로 유지된다.
  - `apps/web/src/components/feed/FieldFilter.tsx:31` (메인 피드 미디어 타입 필터)
  - `apps/web/src/components/feed/TagFilter.tsx:44` (메인 피드 인기 태그 칩)
  - `apps/web/src/app/search/SearchClient.tsx:237` (검색 페이지 카테고리 탭)
  - `apps/web/src/app/search/SearchClient.tsx:255` (검색 페이지 인기 태그 칩)
  - `apps/web/src/app/pin/new/PinCreateForm.tsx:541` (핀 등록 폼 태그 카테고리 탭)
- **의존성**: 추가 없음.
- **API·DB**: 영향 없음.
- **롤백**: 단일 커밋 git revert.
