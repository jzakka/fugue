## Context

DESIGN.md L26-35는 typography scale을 다음과 같이 명시한다:

- L33 `xs: 12px / 0.75rem — creator name, meta`
- L34 `2xs: 11px / 0.6875rem — timestamps, duration`
- L35 `3xs: 10px / 0.625rem — tags, category labels`

각 라벨은 의미 카테고리(creator name, timestamp, tag 등)에 typography 토큰을 명시적으로 매핑한다. `apps/web/src/app/pins/[id]/page.tsx:152`는 핀 상세 페이지의 태그 칩 렌더링 위치인데, 태그 라벨에 `text-xs`(12px / "creator name, meta" 카테고리)를 적용하고 있어 L35 명세와 어긋난다.

동일 카테고리인 `PinCard.tsx:197`은 archive/2026-05-15-text-scale-tokens-2xs-3xs(PinCard.tsx `text-[10px]` → `text-3xs` 정렬 수행)에서 이미 `text-3xs`로 정렬되었다. 핀 상세 페이지는 그 정렬에서 누락된 잔여 1건이다. 토큰 `--text-3xs: 0.625rem`은 `apps/web/src/app/globals.css` `@theme inline`에 archive/2026-05-15-text-scale-tokens-2xs-3xs에서 정의되어 즉시 사용 가능하다.

## Goals / Non-Goals

**Goals:**
- 핀 상세 페이지 태그 칩의 typography 토큰을 DESIGN.md L35 'tags, category labels' 카테고리(3xs)에 정렬.
- 변경 범위를 단일 단어 교체(`text-xs` → `text-3xs`)로 한정하여 effort=1 / risk=1 유지.

**Non-Goals:**
- 태그 칩의 다른 시각 속성(padding `px-2.5 py-1`, 배경 `bg-accent-subtle`, 텍스트 색 `text-text-muted`, 둥근 모서리 `rounded-full`, 폰트 패밀리 `font-mono`)은 변경하지 않는다.
- 다른 핀 상세 페이지 요소(creator name `text-xs` L77 / timestamp `text-xs` 등)는 별도 카테고리이거나 별도 후보이므로 범위 외.
- `PinCard.tsx`와의 padding/spacing 통일은 별도 후보(`px-2.5 py-1` vs `px-2 py-0.5` 차이) — 본 변경 범위 외.

## Decisions

### Decision 1: `text-xs` → `text-3xs` 단일 utility 교체

**Rationale:** DESIGN.md L35가 'tags, category labels' 카테고리에 3xs를 명시적으로 매핑하고, 토큰 `--text-3xs: 0.625rem`이 globals.css에 이미 정의되어 있어 1단어 교체만으로 명세 정렬이 완료된다.

**Alternatives considered:**
- 정적 태그 칩 유지(`text-xs`) → DESIGN.md L35 직접 위반. PinCard 정합 사례와도 불일치.
- 새 토큰 도입 → 이미 `--text-3xs` 존재. 불필요한 토큰 추가는 anti-patterns L16의 'scale 의미 덮어쓰기' 영역과 인접한 위험.
- `text-[10px]` 매직값 → DESIGN.md 토큰 회피로 디자인 시스템 일관성 깨짐.

### Decision 2: 변경 위치를 라인 152 단일 className 토큰 단어로 한정

**Rationale:** anti-patterns L15가 "스케일을 단일 항목으로 묶으면 광범위 시각 회귀를 트리거하므로 후자가 effort 추정을 무너뜨림"을 명시. 본 변경은 신규 토큰 추가가 아니라 기존 토큰 사용처 1곳을 명세 카테고리로 옮기는 정렬이라 anti-pattern과 구조가 다르고 회귀 범위가 단일 화면 단일 요소로 한정된다.

**Alternatives considered:**
- 핀 상세 페이지 전체 typography 일제 점검 → 단발 정렬 사이클 범위(effort=1) 초과. 별도 후보로 분리.

## Risks / Trade-offs

- [Risk] 태그 글자가 작아져(12px → 10px) 가독성 저하 우려 → DESIGN.md L35가 3xs(10px)를 'tags, category labels' 카테고리에 직접 명시하고 있어 명세 결정이며, `PinCard.tsx:197`이 동일 크기로 이미 운영 중. 가독성 정책 결정은 디자인 트랙 루프 범위 밖.
- [Risk] 태그가 많은 핀에서 칩 너비가 줄어들어 레이아웃 점프 → `flex flex-wrap gap-2` 컨테이너는 그대로이고, `px-2.5 py-1` padding도 유지되므로 칩 자체 박스 크기 변동은 글자 폭에 의한 미세 차이만 발생. wrap 컨테이너가 흡수.
- [Trade-off] PinCard(`px-2 py-0.5`) vs 핀 상세(`px-2.5 py-1`) padding 차이는 남는다 → 본 변경 범위 외. typography 카테고리 정렬만 처리하고 padding 통일은 별도 후보 평가.

## Migration Plan

1. `apps/web/src/app/pins/[id]/page.tsx:152` className `text-xs` → `text-3xs` 교체.
2. Next.js dev 서버에서 `/pins/<id>` 페이지 진입, 태그 칩 글자 크기 시각 확인.
3. 회귀 영역: 핀 상세 페이지 1화면. 다른 화면 영향 없음(`pins/[id]/page.tsx` 단일 파일 단일 라인).
4. Rollback: 단일 커밋 git revert.
