## Context

DESIGN.md L54-65 Spacing 섹션은 Base unit 4px + 8단계 의미 라벨 스케일(`2xs: 2px / xs: 4px / sm: 8px / md: 16px / lg: 24px / xl: 32px / 2xl: 48px / 3xl: 64px`)을 명시. 코드베이스에서 spacing 토큰화는 globals.css `@theme inline` 블록에 별도 토큰을 두지 않고 Tailwind v4 기본 spacing scale(`--spacing: 0.25rem` 기반, `gap-0.5` `gap-1` `gap-2` `gap-4` 등 0.25rem 단위 동적 생성)을 그대로 사용한다.

매직 spacing 리터럴(`gap-[2px]`, `p-[12px]` 등)을 코드베이스에서 grep으로 확인한 결과, `gap-[2px]` 2건만 매칭:

- `apps/web/src/components/board/BoardCover.tsx:30` — 보드 cover 2x2 미니어처 그리드 셀 간 gap. `grid grid-cols-2 grid-rows-2 gap-[2px]` 컨테이너의 일부.
- `apps/web/src/components/feed/PinCard.tsx:32` — 오디오 카드 waveform 12개 바 사이 gap. `flex items-end gap-[2px] h-12 mb-4` 컨테이너의 일부.

두 곳 모두 시각 의도가 '얇은 구분선'이나 '바 사이 최소 간격' 같은 minimal 간격이라 DESIGN.md L58 '2xs: 2px' spacing 카테고리 정합. Tailwind v4 기본 `gap-0.5` 클래스가 0.5 × 0.25rem = 0.125rem = 2px로 매직값과 동일 값을 생성하므로 토큰 회수가 가능.

## Goals / Non-Goals

**Goals:**

- 두 곳 `gap-[2px]` 매직값을 Tailwind v4 기본 `gap-0.5` 토큰으로 회수해 코드베이스에서 spacing 매직 리터럴을 0으로 만든다.
- DESIGN.md L58 '2xs: 2px' spacing 카테고리 매핑이 코드에 명시화되도록 한다.

**Non-Goals:**

- `globals.css @theme inline`에 `--spacing-2xs` 같은 별도 spacing 토큰 추가는 본 변경 범위 밖. Tailwind v4 기본 spacing scale을 그대로 사용해 base-class 의미 덮어쓰기를 피한다 (anti-pattern L15).
- DESIGN.md 다른 spacing 카테고리(`xs: 4px ~ 3xl: 64px`)의 일괄 토큰화는 본 변경 범위 밖. 사용처가 광범위(`gap-4` `p-6` `mb-8` 등 수십 곳)이라 별도 사이클이 필요.
- `rounded-[16px]` 같은 다른 카테고리의 매직값 회수는 본 변경 범위 밖 (radius 카테고리는 별도 검토 필요).

## Decisions

### Decision 1: `gap-[2px]` → `gap-0.5` 1단어 교체

`apps/web/src/components/board/BoardCover.tsx:30`과 `apps/web/src/components/feed/PinCard.tsx:32` 두 곳 className에서 `gap-[2px]` 부분만 `gap-0.5`로 교체. 나머지 className utility(예: BoardCover의 `w-full aspect-square rounded-[10px] overflow-hidden grid grid-cols-2 grid-rows-2 ... border ...`)는 그대로 유지.

**이유**: Tailwind v4 기본 spacing scale의 `0.5` 단계는 0.5 × 0.25rem = 0.125rem = 2px로 매직값과 동일 값을 생성. CSS 렌더링 결과 동일. 매직 리터럴 회수만으로 SSoT 의도(DESIGN.md spacing scale 카테고리) 명시화.

**대안 검토**:
- `globals.css @theme inline`에 `--spacing-2xs: 2px` 토큰 추가 + 코드에서 `gap-2xs` 사용: Tailwind v4의 `--spacing-*` 토큰은 base `--spacing` 단위를 덮어쓰는 메커니즘이라 단일 단계만 정의할 수 없고, `--spacing-2xs`처럼 라벨 단계를 추가하려면 전체 spacing scale 라벨 시스템을 도입해야 한다. 광범위 회귀 위험으로 본 사이클 범위 초과.
- `gap-[0.125rem]` 같은 rem 단위 매직값: 매직 리터럴이 그대로 남아 SSoT 회수 목적 미달성.

### Decision 2: 다른 utility는 그대로 유지

BoardCover.tsx:30의 `rounded-[10px]` 같은 다른 매직값은 본 변경에서 손대지 않는다.

**이유**: 본 변경의 범위는 `gap-[2px]` 매직값 회수에 한정. radius 카테고리(`rounded-[10px]`)는 DESIGN.md L75 'md: 10px (cards)' 직접 매핑이지만 별도 검토 필요(`rounded-md` Tailwind v4 기본 0.375rem=6px와 충돌 → base-class 덮어쓰기 검토 필요). 범위 확대는 anti-pattern L15 회피 원칙에 위배.

## Risks / Trade-offs

- **CSS 변환 결과가 매직값과 다를 가능성**: Tailwind v4 `gap-0.5` 클래스가 실제로 `gap: 0.125rem`을 생성하는지 검증 필요. → 완화: Tailwind v4 공식 문서 및 기존 코드베이스에서 `gap-0.5` 사용 사례가 없어도 표준 `--spacing` 토큰 기반 동적 생성 규칙이라 안정적. 시각 비교 검증으로 보조.
- **DESIGN.md 명세와 Tailwind 토큰 라벨 불일치**: DESIGN.md는 의미 라벨(`2xs`)을 사용하나 Tailwind 기본 토큰은 숫자 라벨(`0.5`)을 사용. 코드에 직접적 SSoT 라벨 일치는 안 됨. → 완화: 본 변경의 목적은 '매직 리터럴 회수'이며 '라벨 일치'는 별도 사이클(전체 spacing scale 토큰화)이 필요. anti-pattern L15 회피를 위해 base-class 덮어쓰기는 본 사이클 범위 밖.

## Migration Plan

1. `apps/web/src/components/board/BoardCover.tsx:30` className에서 `gap-[2px]` → `gap-0.5` 1단어 교체.
2. `apps/web/src/components/feed/PinCard.tsx:32` className에서 `gap-[2px]` → `gap-0.5` 1단어 교체.
3. `grep -rnE 'gap-\[2px\]' apps/web/src` 결과가 0건임을 확인.
4. 시각 검증: 보드 cover 미니어처 그리드 및 오디오 카드 waveform 바 사이 간격이 동일 2px로 렌더링됨을 시각 비교.
5. 롤백: 단일 커밋이므로 `git revert <commit>` 실행으로 즉시 원복 가능.
