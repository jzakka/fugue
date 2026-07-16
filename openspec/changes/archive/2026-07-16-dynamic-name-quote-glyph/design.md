# Design: dynamic-name-quote-glyph

## Context

가시 카피의 동적 이름 인용 표면 전수 census(cycle 3847):

| 표면 | 위치 | 글리프 | 채널 |
|------|------|--------|------|
| 검색 결과 헤딩 | SearchClient.tsx:233 (`&ldquo;{query}&rdquo;`) | 곡선 “ ” | 가시 카피 |
| 빈 검색 결과 메시지 | SearchClient.tsx:413 (`` `“${query}”…` ``) | 곡선 “ ” | 가시 카피 |
| 보드 추가 성공 피드백 | AddToBoardButton.tsx:194 (`` `"${boardName}" …` ``) | 직선 " | 가시 카피 |
| 검색 메타 title/description | search/page.tsx:23/:25 | 직선 " | head 메타(비가시) |

## Goals / Non-Goals

- Goal: 가시 카피 채널 내 동적 이름 인용 글리프를 지배 관례(곡선따옴표)로 정렬한다.
- Non-Goal: head 메타데이터 인용 표기 변경, 다른 카피 문형 변경, 인용 유틸 공통화.

## Decisions

### Decision 1: AddToBoardButton:194 단일 지점에서 글리프만 교체한다

```tsx
// before
message: `"${boardName}" 보드에 추가했습니다`,
// after
message: `“${boardName}” 보드에 추가했습니다`,
```

- 대안 A(SearchClient를 직선따옴표로 역정렬): 기각 — 지배 관례(2곳)가 곡선이며, 곡선따옴표는 인용 전용 글리프로 코드 문자열 리터럴 구분자와도 시각적으로 구별된다.
- 대안 B(인용 포맷 헬퍼 공통화): 기각 — 표면 3곳·1행 수정에 추상화는 과잉(c3835 Decision 1 동형).

### Decision 2: head 메타데이터(search/page.tsx:23/:25)는 유지한다

메타 title/description 은 페이지에 렌더되지 않는 plain-text 채널로 가시 카피 모집단 밖. 변경 시 스코프 초과.

## Risks / Trade-offs

- 리스크 없음에 가까움: 텍스트 글리프 2자 교체, 로직·레이아웃 무영향. 기존 문자열을 단정하는 테스트가 있으면 갱신 필요(vitest 확인).
