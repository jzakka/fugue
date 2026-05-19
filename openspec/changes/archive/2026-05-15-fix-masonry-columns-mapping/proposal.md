## Why

`DESIGN.md` L67-70은 피드 그리드 명세를 다음과 같이 정의한다:

- L68 "Approach: Masonry grid"
- L69 "Grid: 4 columns (desktop), 3 (tablet), 2 (mobile), 1 (small mobile)"
- L70 "Breakpoints: sm: 500px, md: 800px, lg: 1200px"

따라서 의도된 매핑은 (Tailwind 관례에 따라 min-width 기준 해석):

| viewport | 영역 | columns |
|---|---|---|
| ≥1200 | desktop | 4 |
| 800~1199 | tablet | 3 |
| 500~799 | mobile | 2 |
| <500 | small mobile | 1 |

그러나 `apps/web/src/components/feed/MasonryGrid.tsx:6-11`은 다음과 같이 매핑한다:

```js
const BREAKPOINT_COLUMNS = {
  default: 4,
  1200: 4,
  800: 3,
  500: 2,
};
```

`react-masonry-css`의 `breakpointCols` 키는 max-width 임계(viewport≤key일 때 해당 컬럼 수 적용)다. 따라서 실제 동작은:

| viewport | 코드 동작 | DESIGN.md 명세 | 차이 |
|---|---|---|---|
| ≥1201 | 4 | 4 | OK |
| 801~1200 | 4 (1200 키) | tablet=3 | tablet에서 4 columns로 오버 |
| 501~800 | 3 (800 키) | mobile=2 | mobile에서 3 columns로 오버 |
| ≤500 | 2 (500 키) | small mobile=1 | small mobile에서 2 columns 유지, 1 누락 |

모든 영역이 한 단계씩 어긋나 있고 small mobile(1 column) 분기가 누락되어 있다. small mobile(아이폰 SE 등 ≤375px)에서 카드가 너무 좁아 미디어 작품의 시인성이 떨어진다.

## What Changes

`apps/web/src/components/feed/MasonryGrid.tsx`의 `BREAKPOINT_COLUMNS`를 DESIGN.md 명세에 맞춰 교체한다:

```js
const BREAKPOINT_COLUMNS = {
  default: 4,    // viewport ≥ 1200 (desktop)
  1199: 3,       // viewport ≤ 1199 (tablet, <1200)
  799: 2,        // viewport ≤ 799 (mobile, <800)
  499: 1,        // viewport ≤ 499 (small mobile, <500)
};
```

키 값을 `1199/799/499`(off-by-one)로 둔 이유: `react-masonry-css`는 키를 max-width inclusive(`viewport ≤ key`)로 해석하므로, "viewport=500은 mobile(2 columns)에 속해야 한다"는 DESIGN.md L70(sm: 500px = mobile breakpoint 시작점)을 정확히 반영하려면 -1 표기가 정확하다.

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: `apps/web/src/components/feed/MasonryGrid.tsx` 단일 파일, 객체 키 4개.
- 사용자 영향: 피드/검색/보드/프로필의 masonry 사용처에서 tablet·mobile·small mobile viewport의 컬럼 수가 DESIGN.md 명세대로 바뀐다. desktop은 동일.
- 사용처: `apps/web/src/components/feed/MasonryGrid.tsx`를 import하는 페이지 — feed, search, board detail, profile 등. MasonryGrid 컴포넌트 인터페이스는 변경 없음(children props 그대로).
- 의존성·DB·인프라 마이그레이션 없음.

## Rollback

- 객체 4개 키를 이전 값(`default:4, 1200:4, 800:3, 500:2`)으로 복원하거나 git revert. 다른 파일 변경 없음.
