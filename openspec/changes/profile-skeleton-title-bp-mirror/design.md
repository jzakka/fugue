# Design: profile-skeleton-title-bp-mirror

## Context

`creators/[id]/loading.tsx`(cycle 3717, PR #4670 신설)가 렌더하는 ProfileSkeleton은 실 ProfileHeader의 기하를 미러하여 스켈레톤→실물 스왑 시 점프를 방지하는 것이 확립 관례다. BP 전이 5건 중 제목 바만 정적(`h-8`)으로 남아 ≥sm에서 4px 어긋난다.

- 실물: `ProfileHeader.tsx:32` `text-2xl sm:text-3xl` — Tailwind 기본 스케일에서 text-2xl 라인박스 2rem(32px), text-3xl 라인박스 2.25rem(36px). 프로젝트 `@theme inline`은 색 토큰·`--font-display`/`--font-mono`·`--text-2xs`/`--text-3xs`·`--shadow-card-hover`만 정의하므로 text-2xl/3xl과 sm BP(640px)는 Tailwind 기본값이 적용된다.
- 스켈레톤: `ProfileSkeleton.tsx:9` `h-8`(2rem=32px) 정적.

## Goals / Non-Goals

**Goals**:
- 스켈레톤 제목 바 높이가 실 h1 라인박스의 BP 전이(32→36px)를 미러한다.

**Non-Goals**:
- 타이포그래피 스케일 토큰 매핑 변경 (사용자 결정으로 별도 관리 — decision-log의 typography-scale-unmapped 결정 침범 금지).
- ProfileSkeleton의 다른 행(아바타, 핀 카운트 바, works 그리드) 변경 — 이미 정합.
- shimmer 모션·착색 변경.

## Decisions

### Decision 1: `h-8` → `h-8 sm:h-9` 단일 모디파이어 추가

```tsx
// ProfileSkeleton.tsx:9
<div className="h-8 sm:h-9 bg-surface-elevated rounded w-48" />
```

- `h-9` = 2.25rem = 36px로 text-3xl 라인박스와 정확히 일치.
- 실물과 동일한 `sm:` BP를 사용하므로 전환 시점이 h1의 타이포 전이와 동기화된다.

**대안 검토**:
- (기각) 스켈레톤 바에 `text-2xl sm:text-3xl` + 불가시 텍스트로 라인박스를 자연 유도 — 마크업 복잡도 증가, 기존 h-* 바 idiom(핀 카운트 h-5, works 제목 h-4)과 어긋남.
- (기각) 실 h1을 정적 크기로 낮춤 — 기존 시각 동작 파괴(§5 보수 정의 (a) 위반), 범위 초과.

### Decision 2: 너비(w-48)와 여백(mt-4 등)은 불변

너비는 실 닉네임 길이가 가변이라 미러 대상이 아니며(기존 결정 유지), 여백 구조는 이미 정합이다. 변경 폭을 클래스 문자열 1건으로 최소화한다(§5 보수 정의 (d)).

## Risks / Trade-offs

- **리스크 최소**: ProfileSkeleton은 `creators/[id]/loading.tsx` 단일 소비처. 다른 화면 회귀 불가능.
- **트레이드오프**: <sm에서는 기존과 동일(32px)하므로 시각 변화 없음. 롤백은 `sm:h-9` 제거 1건.
