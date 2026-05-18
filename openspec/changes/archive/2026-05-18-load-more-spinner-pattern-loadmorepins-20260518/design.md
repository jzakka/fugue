## Context

`apps/web/`의 페이지네이션 로딩 표시 사용처 4곳을 측정한 결과:

| 사용처 | 트리거 | 로딩 표시 |
|--------|--------|-----------|
| `apps/web/src/components/feed/FeedContainer.tsx:212` | Intersection observer auto-load | spinner (w-6 h-6, accent border) |
| `apps/web/src/components/profile/PinsGrid.tsx:135` | Intersection observer auto-load | spinner (w-6 h-6, accent border) |
| `apps/web/src/app/search/SearchClient.tsx:411` | "더보기" 버튼 manual-click 후 버튼 내 표시 | spinner (w-5 h-5, accent border, mx-auto) |
| `apps/web/src/app/boards/[id]/LoadMorePins.tsx:46` | "더보기" 버튼 manual-click 후 버튼 내 표시 | "불러오는 중..." 텍스트 |

3개 spinner vs 1개 텍스트로 코드베이스 측정 75% 패턴이 spinner. manual-click 카테고리 안에서도 SearchClient(spinner) vs LoadMorePins(텍스트)로 비대칭.

DESIGN.md L94 'Skeleton loading: 카드 자리에 shimmer 효과'는 카드 자리(initial load) 한정 명시. 페이지네이션 로딩 표시는 DESIGN.md 직접 명세 외 영역이지만 코드베이스 측정 패턴 outlier 정렬은 archive/2026-05-15-cancel-button-disabled-opacity(disabled:opacity-50 outlier 정렬 사례)와 동일 논리로 처리 가능.

이전 직접 관련된 결정 이력은 없다 (`.fugue/decision-log.md` 기준).

## Goals / Non-Goals

**Goals:**
- LoadMorePins.tsx의 "더보기" 버튼 manual-click 로딩 표시를 코드베이스 75% 패턴(spinner)에 맞춘다.
- 정렬 기준은 같은 manual-click 카테고리 사용처인 SearchClient L411 패턴(`<div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto" />`).
- 버튼 className·padding·disabled 동작·기타 마크업은 변경하지 않는다. JSX 자식 표현식만 교체.

**Non-Goals:**
- DESIGN.md 자체에 페이지네이션 로딩 표시 명세 추가는 본 변경 범위 밖.
- 다른 3곳(FeedContainer·PinsGrid·SearchClient) 로딩 표시 변경은 본 변경 범위 밖. 그것들이 정렬 기준점.
- spinner 크기·색·border-width 변경은 본 변경 범위 밖. SearchClient L411과 동일하게 둔다.
- auto-load(Intersection observer) 패턴의 spinner를 manual-click 패턴과 통일하는 작업은 본 변경 범위 밖. 두 패턴 카테고리 분리 유지.

## Decisions

### Decision 1: SearchClient L411 패턴을 정렬 기준점으로 사용

LoadMorePins와 SearchClient 둘 다 manual-click 카테고리(사용자가 "더보기" 버튼을 명시적으로 클릭한 후 로딩 in-flight)이므로 시각 정합 비교가 정확하다. SearchClient L411의 spinner 패턴:

```tsx
{loadingMore ? (
  <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto" />
) : (
  "더보기"
)}
```

- `w-5 h-5` (20px × 20px) — 버튼 내 inline 크기. auto-load의 w-6 h-6(24px)보다 작아 버튼 안에서 시각 균형 적합.
- `border-2 border-accent border-t-transparent rounded-full animate-spin` — Tailwind 표준 spinner 패턴.
- `mx-auto` — 버튼 width가 텍스트 "더보기"보다 클 때 가로 중앙 정렬.

**대안 1 — auto-load 패턴(w-6 h-6) 사용:**
- 장점: 코드베이스 다수(2곳)의 크기.
- 단점: auto-load는 버튼 외 별도 영역에 표시되므로 24px가 적합하나, manual-click은 버튼 내 표시라 20px가 시각 균형 적합. 카테고리 분리가 옳음.

**대안 2 — 텍스트 그대로 유지:**
- 장점: 명시적 텍스트로 진행 상황을 직관적으로 알림.
- 단점: 코드베이스 다른 manual-click 사용처(SearchClient)와 비대칭. 사이클 25(disabled:opacity-50 outlier 정렬) 논리와 일치 안 함.

**선택**: SearchClient L411 패턴 동일 적용.

### Decision 2: `loading=false` 시 텍스트는 그대로 "더보기"

기존 코드에서 `loading=false` 시 "더보기" 텍스트 표시. 변경 후에도 동일하게 "더보기". SearchClient L411 같은 텍스트 사용.

### Decision 3: 버튼 자식 표현식만 교체, 다른 마크업·className 미변경

LoadMorePins.tsx L42-47 버튼 JSX:
```tsx
<button
  onClick={loadMore}
  disabled={loading}
  className="px-6 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent transition-colors disabled:opacity-50 cursor-pointer"
>
  {loading ? "불러오는 중..." : "더보기"}
</button>
```

`className`(px-6 py-2.5 등)이 SearchClient L408의 `className`(px-6 py-3 bg-surface ...)과 다르지만 본 변경 범위 밖. 버튼 외관 자체의 일관성 작업은 별도 후보(스타일 정렬 vs 행동 정렬 분리).

## Risks / Trade-offs

- **[Risk] DESIGN.md 명세 외 영역에서 코드베이스 측정 패턴에 의존한 정렬이 자의적 해석으로 비판받을 수 있음** → Mitigation: archive/2026-05-15-cancel-button-disabled-opacity(16곳 중 13곳 적용·3곳 잔여 disabled:opacity-50 정렬) 사례가 동일 논리로 처리됨. 본 변경도 같은 논리 일관 적용. confidence 3로 책정(DESIGN.md 명시 위반이 아니라 코드 측정 패턴 outlier 정렬). 보수적 처리로 단일 outlier 1건 한정.
- **[Risk] manual-click 카테고리 안에서만 보면 SearchClient(spinner) vs LoadMorePins(텍스트)로 1:1라 outlier가 아닐 수 있음** → Mitigation: 전체 페이지네이션 로딩 4곳 중 3곳이 spinner라 코드베이스 측정 75% 다수 패턴. SearchClient·LoadMorePins 둘만 한정 비교가 아니라 4곳 전체 비교가 정합 기준. 같은 manual-click 카테고리 SearchClient가 spinner로 정렬되어 있어 LoadMorePins도 같은 패턴 적용이 일관성 측면에서 자연.
- **[Trade-off] spinner는 텍스트보다 진행 상황 직관성이 낮음(시각 신호로만 알림)** → 다만 페이지네이션 로딩은 통상 짧은 시간(1-2초)이라 텍스트 vs spinner 직관성 차이가 크지 않음. 코드베이스 다수 패턴인 spinner 유지가 시각 일관성 우선.
- **[Trade-off] "불러오는 중..." 텍스트가 a11y 측면에서 스크린리더에 명시적으로 읽힘(spinner는 시각 신호만)** → 다만 버튼 자체의 `disabled={loading}`이 변경 없이 유지되어 스크린리더가 disabled 상태를 인식. aria-busy 같은 명시적 a11y attribute 추가는 본 변경 범위 밖(다른 3곳도 미사용). a11y 측면 정렬은 별도 후보로 분리.

## Migration Plan

- **배포**: 단일 커밋. TSX 1줄 변경, 빌드 변경 없이 즉시 반영.
- **롤백**: `git revert <commit>` 으로 단일 커밋 되돌리기.
- **시각 검증**: 보드 상세 페이지의 "더보기" 버튼을 클릭하여 로딩 in-flight 동안 spinner가 표시되는지 확인 (본 변경 범위 안에서 수동 시각 검증).
