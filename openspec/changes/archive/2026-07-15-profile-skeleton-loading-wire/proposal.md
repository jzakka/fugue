# profile-skeleton-loading-wire

## Why

크리에이터 공개 프로필 페이지(`/creators/[id]`)는 서버에서 크리에이터·핀·보드 3건을 순차 fetch(`force-dynamic`)한 뒤에야 렌더되는데, route-level 로딩 UI가 없어 클라이언트 내비게이션 중 사용자는 아무 피드백 없이 이전 화면에 머문다. 한편 이 화면 전용으로 저작된 `ProfileSkeleton` 컴포넌트는 탄생 커밋(6adbf9bf)부터 어디에서도 참조되지 않는 고아 상태다(참조 grep 0건, `apps/web/src/app` 전체 `loading.tsx` 0건). 스켈레톤 로딩은 코드베이스 확립 관례(DESIGN.md Motion "Skeleton loading: 카드 자리에 shimmer 효과", `CardSkeleton`은 FeedContainer·SearchClient·PinsGrid 3표면 배선)인데 프로필 표면만 저작되고 미배선이다.

부수 정합: `ProfileSkeleton`의 헤더 블록에 실제 `ProfileHeader`에 존재하지 않는 phantom 행(bio 자리 바 `h-4 w-72` + 칩 2개 `h-6 rounded-full`)이 있어, 배선 시 스켈레톤↔실콘텐츠 미러 원칙(기존 baseline들이 확인한 "스켈레톤이 실 헤더 크기 정확 미러")에 맞게 실 앵커(제목 h1 + 핀 카운트 행)로 정합해야 한다.

## What Changes

1. **`apps/web/src/app/creators/[id]/loading.tsx` 신설**: Next.js route-level loading 컨벤션(fork docs `loading.md` 지원 확인)으로 `ProfileSkeleton`을 배선. 실 페이지 구조를 미러 — NavBar 자리 스켈레톤(sticky nav 박스: 로고 사각+검색 pill+액션 원형, `skeleton-shimmer` 어휘) + 실 페이지와 동일한 `main` 컨테이너(`flex-1 max-w-4xl mx-auto w-full px-6 py-8`) 안에 `ProfileSkeleton`.
2. **`ProfileSkeleton.tsx` phantom 행 정합**: 헤더 info 블록을 실 `ProfileHeader` anatomy(h1 제목 + `mt-4` 핀 카운트 행)로 정렬 — bio 바·칩 2개 제거, 핀 카운트 자리 바(`mt-4 h-5 w-20`)로 대체.
3. `unstable_instant`는 **미적용**: cacheComponents 비활성(next.config.ts) 환경의 classic 모드에서 loading.tsx 만으로 내비게이션 즉시 fallback이 동작하며, 보수 원칙(공개 API/설정 비변경) 적용.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `profile`: 공개 프로필 화면 진입 시 데이터 로드 중 스켈레톤 로딩 상태를 표시한다 (요구사항 추가)

## Impact

- 신규: `apps/web/src/app/creators/[id]/loading.tsx`
- 수정: `apps/web/src/components/profile/ProfileSkeleton.tsx` (헤더 info 블록 3행→2행)
- 비영향: `creators/[id]/page.tsx`·`ProfileHeader`·`NavBar`·`CardSkeleton` 및 여타 표면 무변경. 로딩 완료 후 최종 렌더 결과 불변(로딩 중 fallback 추가만). 롤백 = loading.tsx 삭제 + 스켈레톤 원복.
