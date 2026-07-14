# Proposal: pinsgrid-filter-active-fill-vocab

## Why

aria-pressed 선택 칩 9표면 전수 census 결과 활성 채움 어휘는 role 로 이분되어 있다 — 태그 칩(PinCreateForm:622·SearchClient:274·TagFilter:66)은 액센트 채움 `bg-accent text-white` + 비활성 `bg-accent-subtle`(DESIGN.md L41 "Accent subtle — 태그 배경, 선택 상태" 명시 role), 타입/카테고리 뷰-필터(FieldFilter:39·SearchClient:249·PinCreateForm:586·:600)는 반전 채움 `bg-text-primary text-bg` + 비활성 surface/transparent border. PinsGrid(프로필/마이페이지 미디어타입 필터, PinsGrid.tsx:104-119)만 하이브리드다: 비활성은 뷰-필터 role(`bg-surface border border-border`)인데 활성은 태그 role 의 `bg-accent text-white`, hover 도 `hover:border-accent`(뷰-필터 확립형 `hover:border-text-muted` 이탈)이며 `py-2`·`font-medium` 누락. FieldFilter 는 동일 MEDIA_TYPES(전체/이미지/음악/영상) 라벨의 정확한 동종 비교쌍으로, 같은 콘텐츠 필터가 피드에서는 반전, 프로필에서는 액센트로 렌더돼 cross-surface 활성-채움 어휘가 깨진다.

## What Changes

- `apps/web/src/components/profile/PinsGrid.tsx` 미디어타입 필터 버튼 className 을 FieldFilter 와 동일한 뷰-필터 어휘로 정합화:
  - 활성: `bg-accent text-white` → `bg-text-primary text-bg`
  - 비활성 hover/focus-visible: `hover:border-accent`/`focus-visible:border-accent` → `hover:border-text-muted`/`focus-visible:border-text-muted`
  - chip 공통 클래스: `py-2` → `py-1.5`, `font-medium` 추가 (FieldFilter 아키타입 정렬)
  - 비활성 base `bg-surface border border-border` 는 유지 (PinCreateForm:586·:600 카테고리 필터와 동일한 뷰-필터 role 내 확립 변형 — 보수 원칙 (a) 기존 시각 동작 최소 변경)
- 기능(필터링, aria-pressed, IntersectionObserver 로드)·마크업 구조 비변경. 시각 클래스만 교체.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `profile`: 프로필 미디어타입 필터 칩의 선택 시각 어휘 요구사항 추가(활성=반전 채움, 액센트 채움은 태그 role 한정) — 델타 spec 으로 명문화.

## Impact

- 영향 코드: `apps/web/src/components/profile/PinsGrid.tsx` 1파일, className 문자열만.
- 영향 화면: `/creators/[id]` 프로필 페이지, 마이페이지(MyPageClient) — 미디어타입 필터 칩의 활성/hover 시각.
- 롤백: 커밋 revert 1회로 완전 복원. 토큰/공개 API 비변경.
- 근거: DESIGN.md L38 "액센트는 사용자 액션(CTA, 호버, 선택)에만 써서 작품과 경쟁하지 않는다"(Restrained) + L41 액센트-subtle 은 태그 role 명시 → 뷰-필터의 액센트 채움은 태그 role 침범이며 확립 어휘는 반전 채움.
