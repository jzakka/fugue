# Proposal: profile-skeleton-title-bp-mirror

## Why

ProfileSkeleton의 제목 바가 `h-8`(32px) 정적인 반면, 실 ProfileHeader의 h1은 `text-2xl sm:text-3xl`(라인박스 32px→36px) BP 전이를 가진다. ProfileHeader↔ProfileSkeleton 미러 쌍의 BP 전이 5건 census에서 유일하게 미러되지 않은 전이로, ≥sm 뷰포트에서 스켈레톤→실물 스왑 시 제목 영역이 4px 어긋나 핀 카운트 행 이하가 점프한다.

**Evidence**:
- `apps/web/src/components/profile/ProfileSkeleton.tsx:9` — `<div className="h-8 bg-surface-elevated rounded w-48" />` (정적)
- `apps/web/src/components/profile/ProfileHeader.tsx:32` — `<h1 className="text-2xl sm:text-3xl ...">` (라인박스 2rem=32px → 2.25rem=36px)
- BP 전이 census 5건 중 4건 정합: p-6 sm:p-8 (:13↔:5), flex-col sm:flex-row (:14↔:6), 아바타 w-20 h-20 sm:w-24 sm:h-24 (:21/:24↔:7), works grid grid-cols-1 sm:grid-cols-2 (PinsGrid:140/:148↔:15). 제목 바만 누락.

**근거 관례**: cycle 3717 PR #4670이 "실 ProfileHeader 앵커 정합·스왑 기하 점프 방지"를 명시 채택한 머지 결정. DESIGN.md L94(Skeleton loading: 카드 자리에 shimmer 효과)는 기하를 규정하지 않으나, 본 건은 확립된 미러 관례의 잔여 결함이다.

## What Changes

- `apps/web/src/components/profile/ProfileSkeleton.tsx:9`의 제목 바를 `h-8` → `h-8 sm:h-9`로 변경하여 실 h1의 라인박스 BP 전이(32→36px)를 미러한다.

**사용자 영향**: ≥sm 뷰포트에서 프로필 페이지 로딩 스켈레톤→실물 전환 시 제목/핀 카운트 영역의 4px 수직 점프가 사라진다. <sm에서는 변화 없음.

**롤백 절차**: 단일 클래스 문자열 변경이므로 `sm:h-9` 제거로 즉시 롤백 가능.

## Capabilities

### New Capabilities

없음.

### Modified Capabilities

- `profile`: 프로필 로딩 스켈레톤의 제목 자리가 실 제목의 반응형 크기 전이를 미러해야 한다는 요구 추가.

## Impact

- **Affected code**: `apps/web/src/components/profile/ProfileSkeleton.tsx` 1개 파일, 1줄 (클래스 문자열).
- **Affected surfaces**: `creators/[id]/loading.tsx`가 렌더하는 프로필 로딩 화면만. 피드/검색/보드의 CardSkeleton은 별도 컴포넌트로 무영향.
- **QA plan**: (1) ≥sm(1280px) 스켈레톤 제목 바 computed height 36px, 스왑 후 h1 라인박스 36px 일치·핀 카운트 y-오프셋 불변. (2) <sm(500px) 제목 바 32px 유지. (3) shimmer 모션·착색 무변경, 콘솔 에러 0. (4) 인접 회귀: 피드 CardSkeleton 무변경.
