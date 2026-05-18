## Why

`apps/web/src/app/globals.css` `.light` 블록의 색상 토큰 4건이 DESIGN.md(디자인 시스템 SSoT) 명세값과 어긋나, light mode 토글 시 본문/보조 텍스트·카드 경계·태그 배경의 가시성이 명세 의도와 다르게 렌더링된다. Dark mode 토큰 15건은 모두 명세 일치이고 light mode 4건만 잔여로 어긋남.

## What Changes

- `globals.css` `.light` 블록 4줄 값 교체로 light mode 색상 토큰을 DESIGN.md 명세에 정렬한다.
  - `--text-muted`: `#666666` → `#777777` (DESIGN.md L48 'Text muted: #888888 / #777777')
  - `--text-dim`: `#999999` → `#AAAAAA` (DESIGN.md L49 'Text dim: #555555 / #AAAAAA')
  - `--accent-subtle`: `rgba(232, 90, 42, 0.08)` → `rgba(232, 90, 42, 0.12)` (DESIGN.md L41 'Accent subtle: rgba(232, 90, 42, 0.12)' — dark/light 분리 명시 없음, 단일 값)
  - `--border`: `#D0D0D0` → `#E0E0E0` (DESIGN.md L46 'Border: #2A2A2A / #E0E0E0')

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
- `design-tokens`: light mode 색상 토큰값이 DESIGN.md SSoT와 정렬되어, light mode 활성 시 본문·보조 텍스트(text-muted/dim)·카드 경계(border)·태그 배경(accent-subtle)이 명세 의도대로 렌더링되어야 한다는 요구사항을 추가한다.

## Impact

- 변경 파일: `apps/web/src/app/globals.css` (4줄 값 교체)
- 영향 사용처: `text-text-muted` 사용처 ~30곳, `text-text-dim` 사용처 36곳, `border-border` 사용처 다수, `bg-accent-subtle` 사용처 다수. 단 light mode 토글 활성 시에만 시각 적용. dark mode(기본값)는 무영향.
- 컴포넌트 코드 변경 없음. `ThemeToggle.tsx`가 `.light` 클래스를 활성화하면 4건 즉시 사용처에 반영.
- 시각 변화 폭: 4건 모두 17 step 이내 미세 값 변동 + accent-subtle opacity 0.08→0.12 (50% 증가). 큰 시각 회귀 없음.
- 롤백 절차: `git revert <commit>`로 단일 커밋 되돌리기.
