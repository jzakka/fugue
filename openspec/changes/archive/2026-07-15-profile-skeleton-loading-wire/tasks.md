# Tasks — profile-skeleton-loading-wire

## 1. 구현

- [x] 1.1 `apps/web/src/app/creators/[id]/loading.tsx` 신설 — design.md D1/D2 코드대로 NavBar 자리 미러 스켈레톤 + `main id="main"` 컨테이너 + `ProfileSkeleton` 배선
- [x] 1.2 `apps/web/src/components/profile/ProfileSkeleton.tsx` info 블록 정합 — design.md D3대로 phantom 행(bio 바·칩 2개) 제거, `h-8 w-48` 제목 바 + `mt-4 h-5 w-20` 핀 카운트 바 2행으로 교체(`space-y-3`→`mt-4`)

## 2. 검증 (사전 조건)

- [x] 2.1 `cd apps/web && npx tsc --noEmit` 통과
- [x] 2.2 `cd apps/web && npx vitest run` 통과 (기존 47건 회귀 0)

## 3. 실 브라우저 QA

- [x] 3.1 dev 서버에서 피드→크리에이터 프로필 내비게이션 시 스켈레톤 렌더 후 실 콘텐츠 교체 확인 (네트워크 지연 상황 포함)
- [x] 3.2 스켈레톤 anatomy 실 화면 정합(phantom 행 부재·nav 자리 기하)·skeleton-shimmer 모션·콘솔 에러 0 확인
- [x] 3.3 존재하지 않는 id 진입 시 404 경로 회귀 0 확인
- [x] 3.4 인접 회귀: 피드/검색 CardSkeleton 표면 무변경 확인
