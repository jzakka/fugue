## Why

DESIGN.md L94는 스켈레톤 로딩을 명시한다: `Skeleton loading: 카드 자리에 shimmer 효과` (Motion 섹션).

shimmer는 sliding gradient highlight 애니메이션이다. Tailwind `animate-pulse`는 박스 전체의 opacity를 점멸시키는 별개 애니메이션이며, shimmer와 시각적으로 구분된다.

현재 두 스켈레톤이 `animate-pulse`를 사용한다:

- `apps/web/src/components/feed/CardSkeleton.tsx:3` — 카드 외곽 div `className="bg-surface rounded-[10px] overflow-hidden animate-pulse"`.
- `apps/web/src/components/profile/ProfileSkeleton.tsx:3` — 외곽 div `className="animate-pulse space-y-6"`.

두 컴포넌트의 placeholder 박스 형태는 모두 `bg-surface-elevated`로 단일 클래스에 매핑되어 있다(grep 14건 모두 두 파일 내). 따라서 외곽 컨테이너에 shimmer 트리거 클래스를 부여하고, 자식 셀렉터(`.<trigger> .bg-surface-elevated`)로 일괄 적용하면 placeholder별 클래스 수정 없이 14개 박스가 동시에 shimmer를 갖는다.

## What Changes

1. `apps/web/src/app/globals.css`:
   - `:root`와 `.light` 블록에 `--shimmer-highlight` 변수 추가.
     - dark: `rgba(255, 255, 255, 0.06)` (어두운 surface-elevated 위에 미묘한 화이트 하이라이트).
     - light: `rgba(0, 0, 0, 0.04)` (밝은 surface-elevated 위에 미묘한 블랙 하이라이트).
   - `@keyframes shimmer` 정의(background-position을 좌→우로 슬라이드).
   - `.skeleton-shimmer .bg-surface-elevated` 셀렉터에 sliding linear-gradient background-image + animation 부여. 기존 `background-color: var(--color-surface-elevated)`는 Tailwind base 그대로 유지되고, 그 위에 gradient overlay가 흐른다.
2. `apps/web/src/components/feed/CardSkeleton.tsx:3` 외곽 div className: `animate-pulse` → `skeleton-shimmer`.
3. `apps/web/src/components/profile/ProfileSkeleton.tsx:3` 외곽 div className: `animate-pulse` → `skeleton-shimmer`.

설계 결정:
- placeholder 박스 단위로 shimmer를 입혀 "카드 자리에 shimmer가 흐른다"는 DESIGN.md 표현을 가장 직관적으로 구현. 외곽 컨테이너(`bg-surface`) 자체는 shimmer 대상이 아님.
- duration은 `1.5s`로 설정. DESIGN.md L91은 motion duration scale을 `short(150-250ms)` `medium(250-400ms)`로 정의하지만 이는 일회성 트랜지션 기준이며, shimmer는 무한 loop이므로 일반 shimmer 패턴 관행(1.2~2s) 안에서 중간값 선택.
- shimmer highlight 색은 토큰화해 dark/light 모드 자동 매핑.
- 자식 셀렉터 방식이므로 컴포넌트 마크업의 변경 폭이 외곽 div 1줄로 한정됨. 14개 placeholder를 일일이 수정할 필요 없음.

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: globals.css / CardSkeleton.tsx / ProfileSkeleton.tsx 3 파일.
- 사용자 영향: 피드 / 마이페이지 진입 직후 슬로 네트워크에서 카드 자리의 점멸(opacity)이 사라지고, 좌→우로 흐르는 미묘한 하이라이트(shimmer)가 표시된다.
- 다른 화면 영향: `bg-surface-elevated`는 14건 모두 두 스켈레톤 컴포넌트 내부에 있으며, 자식 셀렉터는 `.skeleton-shimmer` 부모 안에서만 작동하므로 다른 곳의 `bg-surface-elevated` 사용에는 영향 없음.
- 레이아웃 시프트: 없음.
- 성능: GPU 가속 background-position transition. 영향 없음.
- 의존성·인프라·DB 마이그레이션 없음.

## Rollback

- 3 파일의 변경 라인을 git revert하면 즉시 이전 상태로 복귀.
