## 1. globals.css 토큰 및 keyframes 추가

- [x] 1.1 `apps/web/src/app/globals.css`의 `:root` 블록(L8-24)에 `--shimmer-highlight: rgba(255, 255, 255, 0.06);` 추가.
- [x] 1.2 `.light` 블록(L26-36)에 `--shimmer-highlight: rgba(0, 0, 0, 0.04);` 추가.
- [x] 1.3 globals.css 파일 하단(masonry 블록 뒤)에 `@keyframes shimmer { 0% { background-position: -200% 0; } 100% { background-position: 200% 0; } }` 추가.
- [x] 1.4 같은 위치에 `.skeleton-shimmer .bg-surface-elevated { background-image: linear-gradient(90deg, transparent 0%, var(--shimmer-highlight) 50%, transparent 100%); background-size: 200% 100%; background-repeat: no-repeat; animation: shimmer 1.5s linear infinite; }` 추가.

## 2. CardSkeleton 클래스 교체

- [x] 2.1 `apps/web/src/components/feed/CardSkeleton.tsx:3` 외곽 div className: `bg-surface rounded-[10px] overflow-hidden animate-pulse` → `bg-surface rounded-[10px] overflow-hidden skeleton-shimmer`.

## 3. ProfileSkeleton 클래스 교체

- [x] 3.1 `apps/web/src/components/profile/ProfileSkeleton.tsx:3` 외곽 div className: `animate-pulse space-y-6` → `skeleton-shimmer space-y-6`.

## 4. 검증

- [x] 4.1 `grep -rn "animate-pulse" apps/web/src/` 결과 0건.
- [x] 4.2 `grep -n "skeleton-shimmer" apps/web/src/components/feed/CardSkeleton.tsx apps/web/src/components/profile/ProfileSkeleton.tsx` 결과 각 1건.
- [x] 4.3 globals.css에 `@keyframes shimmer`, `.skeleton-shimmer .bg-surface-elevated`, `--shimmer-highlight` (`:root`/`.light` 각 1건) 모두 존재.

## 5. 사후 기록

- [x] 5.1 `.fugue/decision-log.md`에 항목 1~3줄 추가.
- [x] 5.2 `.fugue/backlog-design.yaml`에서 `design-20260515-skeleton-pulse-not-shimmer` status를 `done`으로 변경.
