## Why

`DESIGN.md` L37-49는 색 팔레트를 토큰으로 한정한다: `--accent #E85A2A`, `--accent-hover #FF6B3D`, `--accent-subtle rgba(232,90,42,0.12)`. accent 계열에 이 세 가지 외 색은 정의되지 않는다. 또 L38 "Restrained — 액센트는 사용자 액션(CTA, 호버, 선택)에만 써서 작품과 경쟁하지 않는다"가 사용 원칙이다.

그러나 아바타 폴백 그라디언트가 6개 위치에서 팔레트 밖 색을 두 번째 stop으로 쓴다:

- `apps/web/src/components/nav/NavBar.tsx:46` — `from-accent to-orange-400`
- `apps/web/src/components/nav/SearchBar.tsx:322` — `from-accent to-orange-400`
- `apps/web/src/components/feed/PinCard.tsx:170` — inline `linear-gradient(135deg, var(--accent), #FF8A5C)`
- `apps/web/src/components/profile/ProfileHeader.tsx:24` — `from-accent to-orange-400`
- `apps/web/src/app/pins/[id]/page.tsx:214` — `from-accent to-orange-400`
- `apps/web/src/app/search/SearchClient.tsx:325` — `from-accent to-orange-400`

`orange-400`(Tailwind v4 기본값 `#FB923C`)과 `#FF8A5C`는 토큰에 정의되어 있지 않다. globals.css의 `@theme inline`은 `--color-accent-hover: var(--accent-hover)`를 노출하므로 `to-accent-hover` 클래스가 즉시 사용 가능하다.

본 change는 6개 위치의 그라디언트 두 번째 stop을 모두 `accent-hover` 토큰으로 통일한다. 첫 번째 stop은 기존 `accent` 그대로 유지하므로 시각 인상(따뜻한 vermillion → 약간 더 밝은 vermillion)은 거의 동일하다.

## What Changes

- 5개 파일(NavBar / SearchBar / ProfileHeader / pins/[id]/page / search/SearchClient)에서 `to-orange-400` → `to-accent-hover` (className 교체).
- 1개 파일(PinCard) inline 스타일: `linear-gradient(135deg, var(--accent), #FF8A5C)` → className 형태 `bg-gradient-to-br from-accent to-accent-hover`로 일관화. 기존 inline `style` prop 제거.
- 그라디언트 방향(135deg ≡ `bg-gradient-to-br`), 사이즈, 다른 클래스는 변경 없음.

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: 6개 파일, 6 라인.
- 사용자 영향: 아바타 폴백 그라디언트의 두 번째 stop 색이 `#FB923C` / `#FF8A5C` → `#FF6B3D`로 변경. 헥스 거리 작아 시각 차이는 미미(어두운 배경에서 거의 동일한 따뜻한 오렌지).
- 성능 영향 없음.
- 의존성·인프라·DB 마이그레이션 없음.
- 디자인 결정 기록: 아바타 폴백이 "사용자 액션이 아니므로 accent를 쓰지 말아야 하지 않는가"는 별도 논점이지만, 본 change는 기존 결정(accent 사용)을 유지한 채 팔레트 일관성만 회복한다. 그 논점은 별도 후보로 분리해야 함(현 백로그엔 없음).

## Rollback

- 6개 파일 diff를 git revert 또는 토큰을 raw 색으로 되돌리면 즉시 이전 상태로 복귀.
