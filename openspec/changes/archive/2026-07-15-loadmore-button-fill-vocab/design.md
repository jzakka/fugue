# Design: loadmore-button-fill-vocab

## Context

manual load-more 컨트롤 아키타입 3표면 census:

| 표면 | 파일:라인 | 채움 | 세로 패딩 |
|------|-----------|------|-----------|
| 검색 결과 더보기 button | `apps/web/src/app/search/SearchClient.tsx:422` | `bg-surface` | `py-3` |
| 피드 noscript "다음 페이지" anchor | `apps/web/src/components/feed/FeedContainer.tsx:229` | `bg-surface` | `py-3` |
| 보드 상세 더보기 button | `apps/web/src/app/boards/[id]/LoadMorePins.tsx:45` | (없음) | `py-2.5` |

세 표면은 동일 아키타입이다: 목록 하단 `flex justify-center py-8` 컨테이너, `hasMore` 게이트, 동일 스피너(`w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto`), `disabled:opacity-50`. LoadMorePins 는 과거 2회(로딩 텍스트→스피너 정렬, focus-visible 추가) 동일 outlier 논리로 SearchClient 에 정렬된 전력이 있다.

- DESIGN.md L38: 액센트는 사용자 액션에만 — 본 변경은 액센트 미사용(중립 surface 채움)으로 정합.
- DESIGN.md L43: Surface #161616 — 카드 배경. 더보기 버튼 채움은 canonical 2표면이 이미 채택한 어휘.
- anti-patterns L280 예외 조항: 동일 tier·동일 archetype 의 무근거 padding 분기 격리 site 는 등록 가능.

## Goals / Non-Goals

- Goal: LoadMorePins 더보기 버튼을 majority 어휘(`bg-surface` + `py-3`)로 정렬해 cross-surface 시각 일관성 회복.
- Non-Goal: SearchClient·FeedContainer 변경(canonical 표면), 버튼 동작/구조/토큰 변경, 스피너·컨테이너 변경.

## Decisions

### Decision 1: majority 정렬 방향 — LoadMorePins 를 bg-surface+py-3 로

- 대안 A (채택): LoadMorePins 1곳을 2:1 majority 어휘로 정렬. 변경 폭 최소(1줄), canonical 표면 무변경.
- 대안 B (기각): SearchClient·FeedContainer 2곳을 py-2.5·무채움으로 정렬 — 변경 폭 2배, majority 를 outlier 에 맞추는 역방향이며 기존 시각 동작 보존 원칙(보수 §5a) 위반.

### Decision 2: 변경 단위 — className 문자열 내 2개 유틸만

`apps/web/src/app/boards/[id]/LoadMorePins.tsx:45`:

```
- className="px-6 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors disabled:opacity-50 cursor-pointer"
+ className="px-6 py-3 bg-surface border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors disabled:opacity-50 cursor-pointer"
```

`bg-surface` 삽입 위치는 SearchClient:422 와 동일하게 `py-3` 뒤·`border` 앞(어휘 순서까지 미러링). JSX 구조·핸들러·상태·스피너 분기 미변경.

## Risks / Trade-offs

- 위험: 보드 상세 더보기 버튼의 시각이 2px 커지고 배경이 채워짐 — 의도된 변경이며 인접 레이아웃(`flex justify-center py-8` 컨테이너)은 중앙 정렬이라 시프트 없음.
- light 테마: `bg-surface` 는 light 에서 #FFFFFF 로 재정의되므로 SearchClient 와 동일하게 동작(테마 축 회귀 없음).

## Rollback

className 1줄 revert.
