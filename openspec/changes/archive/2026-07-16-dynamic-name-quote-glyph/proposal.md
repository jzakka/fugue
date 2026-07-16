# Proposal: dynamic-name-quote-glyph

## Why

가시 UI 카피에서 사용자 유래 동적 이름(검색어·보드명)을 따옴표로 감싸는 표면이 전수 3곳인데 인용부호 글리프가 갈린다. 검색 결과 헤딩(SearchClient:233 `&ldquo;{query}&rdquo;`)과 빈 검색 결과 메시지(SearchClient:413 `“${query}”`)는 곡선따옴표(“ ”)를 쓰는 반면, 보드 추가 성공 피드백(AddToBoardButton:194 `"${boardName}" 보드에 추가했습니다`)만 직선따옴표(")를 쓴다. 동일 role(동적 이름 인용 안내문)·동일 채널(가시 카피)에서 2:1 실갈림이며, 컴포넌트 경계만으로는 갈림이 설명되지 않는다(경계 결정론 반증). 지배 관례는 곡선따옴표다.

head 메타데이터 2곳(search/page.tsx:23/:25 직선따옴표)은 페이지에 렌더되지 않는 비가시 plain-text 채널로 본 정렬의 모집단 밖이다.

## What Changes

- AddToBoardButton 보드 추가 성공 피드백 메시지의 보드명 인용부호를 직선따옴표(`"${boardName}"`)에서 곡선따옴표(`“${boardName}”`)로 변경한다.
- 그 외 카피(409 중복 안내·실패 안내·검색 표면)는 변경하지 않는다.

## Capabilities

### Modified Capabilities

- `board`: 보드 추가 성공 피드백에서 동적 보드명 인용 표기가 서비스 가시 카피의 지배 인용 표기(곡선따옴표)와 일치해야 한다는 요구사항 추가.

## Impact

- `apps/web/src/components/board/AddToBoardButton.tsx` 1행 (메시지 템플릿 글리프만).
- 시각 채널: 성공 피드백 텍스트 렌더링. 레이아웃·동작·API 무영향.
