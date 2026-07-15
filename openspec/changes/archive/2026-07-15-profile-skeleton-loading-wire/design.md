# Design — profile-skeleton-loading-wire

## Context

- `/creators/[id]`(`apps/web/src/app/creators/[id]/page.tsx`)는 `export const dynamic = "force-dynamic"` + 3건 순차 서버 fetch(fetchCreator→fetchPins→fetchBoards). route-level `loading.tsx`가 앱 전체에 0건이라 클라이언트 내비게이션 중 로딩 피드백이 없다.
- `ProfileSkeleton`(`apps/web/src/components/profile/ProfileSkeleton.tsx`)은 참조 0건 고아. anatomy는 실 화면을 미러하도록 저작됐으나 헤더 info 블록에 phantom 행(bio 바 `h-4 w-72`·칩 `h-6 rounded-full w-16/w-14`)이 있다. 실 `ProfileHeader`는 h1 닉네임 + `mt-4` 핀 카운트 행뿐.
- 스켈레톤 관례: `skeleton-shimmer` 컨테이너 아래 `bg-surface-elevated` 블록에만 shimmer 적용(globals.css:104). `CardSkeleton`이 3표면 배선된 확립 어휘.
- fork Next.js docs(`node_modules/next/dist/docs/.../loading.md`): loading.tsx는 Suspense fallback으로 prefetch되어 내비게이션 즉시 표시. `unstable_instant`는 Cache Components 모드 기능인데 본 프로젝트는 `cacheComponents` 비활성(next.config.ts에 미설정) — 미적용.
- 루트 layout은 정적(runtime data 접근 없음)이므로 loading fallback이 차단되지 않음(docs "Good to know" 케이스 비해당). NavBar는 layout이 아닌 page 안에서 렌더되므로 loading 중에는 NavBar가 없다 → 자리 미러 필요.

## Goals / Non-Goals

**Goals:**
- `/creators/[id]` 내비게이션/직접 진입 시 즉시 스켈레톤 로딩 UI 표시.
- 스켈레톤 anatomy를 실 화면(NavBar 박스 + ProfileHeader + 그리드) 기하에 정합 — 콘텐츠 스왑 시 레이아웃 점프 최소화.
- 기존 시각 동작 보존: 로딩 완료 후 최종 렌더 불변.

**Non-Goals:**
- `unstable_instant`/cacheComponents 도입 (설정 변경 비대상).
- BoardGrid 자리 스켈레톤 추가, PinsGrid 필터 칩 미러 등 anatomy 확장 (영향 범위 최소).
- 다른 라우트(feed·search·boards)의 loading.tsx (각 표면은 이미 컴포넌트 레벨 CardSkeleton 처리 보유).

## Decisions

### D1. route-level `loading.tsx`로 배선 (컴포넌트 Suspense 아님)
`creators/[id]/page.tsx`는 서버 컴포넌트에서 순차 await 하므로 컴포넌트 레벨 Suspense를 넣으려면 페이지 구조 재편이 필요하다. loading.tsx 파일 컨벤션은 페이지 무변경으로 동일 효과(자동 Suspense 래핑) — 보수 원칙(기존 코드 비변경, 롤백 = 파일 삭제).

```tsx
// apps/web/src/app/creators/[id]/loading.tsx (신규)
import ProfileSkeleton from "@/components/profile/ProfileSkeleton";

export default function Loading() {
  return (
    <>
      <header>
        <nav className="sticky top-0 z-50 bg-bg border-b border-border px-6 py-4 flex items-center gap-6 backdrop-blur-sm skeleton-shimmer">
          <div className="flex items-center gap-2 shrink-0">
            <div className="w-8 h-8 bg-surface-elevated rounded-md" />
            <div className="h-6 w-16 bg-surface-elevated rounded" />
          </div>
          <div className="flex-1 max-w-md">
            <div className="h-[42px] bg-surface-elevated rounded-full" />
          </div>
          <div className="ml-auto flex items-center gap-4 shrink-0">
            <div className="w-9 h-9 bg-surface-elevated rounded-full" />
          </div>
        </nav>
      </header>
      <main id="main" className="flex-1 max-w-4xl mx-auto w-full px-6 py-8">
        <ProfileSkeleton />
      </main>
    </>
  );
}
```

### D2. NavBar 자리 미러 스켈레톤 포함
NavBar는 page 내부에서 렌더되므로 loading 중 부재 → 스왑 시 본문이 nav 높이만큼 점프한다. nav 박스(`px-6 py-4 border-b`)와 내부 최고높이 요소(SearchBar input: py-2.5+text-sm+border = 42px)를 미러해 기하 보존. 로고 사각 `w-8 h-8 rounded-md`(실측 동일), 검색 pill `h-[42px] rounded-full`, 액션 원형 `w-9 h-9 rounded-full`(ThemeToggle/아바타 실측 동일). arbitrary `h-[42px]`는 기존 코드 선례 존재(max-h-[480px]·min-h-[3px]). 대안(nav 자리 생략)은 스왑 점프로 기각.

### D3. ProfileSkeleton phantom 행 → 실 앵커 정합
```tsx
// info 블록 (변경 전: space-y-3 + 제목/bio/칩 3행)
<div className="flex-1">
  <div className="h-8 bg-surface-elevated rounded w-48" />   {/* h1 닉네임 (text-2xl≈32px) */}
  <div className="mt-4 h-5 bg-surface-elevated rounded w-20" /> {/* mt-4 핀 카운트 (text-sm≈20px) */}
</div>
```
실 ProfileHeader가 `mt-4`로 간격을 잡으므로 skeleton도 `space-y-3`→`mt-4`로 정렬. 그 외(컨테이너·아바타·works 그리드) 무변경. 대안(phantom 행 유지)은 미러 원칙 위반으로 기각, 대안(BoardGrid 자리 추가)은 Non-Goal.

### D4. `main id="main"` 유지
스킵 링크(`#main`)가 로딩 중에도 유효하도록 실 페이지와 동일한 `main id="main"` 사용. loading과 page는 동시에 렌더되지 않으므로 id 충돌 없음.

## Risks / Trade-offs

- **NavBar 미러 근사치**: 로그인/비로그인 상태별 우측 액션 폭이 다르지만(핀 생성 버튼 유무) 높이는 SearchBar(42px)가 지배해 동일 — 세로 기하만 보장하고 가로 구성은 근사(스켈레톤 특성상 허용).
- **fallback 지속시간이 짧을 때 깜빡임**: 로컬에서 fetch가 빠르면 스켈레톤이 한 프레임만 보일 수 있음 — 기존 CardSkeleton 표면들과 동일한 trade-off로 수용.
- **notFound 경로**: UUID 비정형/미존재 크리에이터는 fallback 표시 후 404로 전환(docs Status Codes 절의 streamed notFound 동작) — 회귀 아님, QA에서 확인.
