## Why

`apps/web/src/app/pins/[id]/page.tsx:152` 핀 상세 페이지의 정적 태그 칩이 `text-xs`(12px, DESIGN.md L33 "xs: 12px — creator name, meta") 카테고리로 렌더링된다. DESIGN.md L35는 "3xs: 10px / 0.625rem — tags, category labels"로 태그 라벨을 명시적으로 3xs 카테고리에 매핑한다. 같은 카테고리의 `PinCard.tsx:197` 태그 칩은 archive/2026-05-15-text-scale-tokens-2xs-3xs(PinCard.tsx에 대해 `text-[10px]` → `text-3xs` 정렬 수행)에서 이미 `text-3xs`로 정렬되었고, 핀 상세 페이지가 이 정렬에서 누락된 1건이다.

## What Changes

- `apps/web/src/app/pins/[id]/page.tsx:152` 태그 칩 `<span>` className `text-xs` → `text-3xs` 1단어 교체.
- 동일 라인의 다른 utility(`px-2.5 py-1 bg-accent-subtle text-text-muted rounded-full font-mono`)는 그대로 유지.
- 표시되는 태그 글자 크기 12px → 10px (2px 감소).

## Capabilities

### New Capabilities
- `design-tokens`: DESIGN.md typography scale(L26-35)에 정의된 텍스트 카테고리(creator name·meta / timestamp+duration / tags+category labels)가 코드 SSoT 내에서 의미 카테고리별로 일관된 시각 위계로 렌더링되도록 보장하는 행위 계약. archive/2026-05-18-creator-card-timestamp-text-2xs-20260518 사례와 동일 capability에 누적.

### Modified Capabilities
(none)

## Impact

- 파일: `apps/web/src/app/pins/[id]/page.tsx` 1라인.
- 시각 변화: 핀 상세 페이지 정적 태그 칩 글자 크기 12px → 10px. font-mono / text-text-muted / bg-accent-subtle / rounded-full 모두 그대로 유지.
- 토큰: `--text-3xs: 0.625rem`는 `apps/web/src/app/globals.css` `@theme inline`에 이미 정의되어 있어 신규 토큰 추가 불필요(archive/2026-05-15-text-scale-tokens-2xs-3xs).
- 의존성: 없음. apps/api / DB / 서드파티 무관.
- 롤백: 단일 커밋 git revert.
