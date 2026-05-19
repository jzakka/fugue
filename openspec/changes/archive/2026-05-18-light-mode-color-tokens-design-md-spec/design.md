## Context

DESIGN.md L37-50 'Color' 섹션은 각 색상 카테고리에 대해 dark mode / light mode 두 값을 슬래시로 분리해 명시한다:

- L46 Border: `#2A2A2A / #E0E0E0`
- L48 Text muted: `#888888 / #777777`
- L49 Text dim: `#555555 / #AAAAAA`
- L41 Accent subtle: `rgba(232, 90, 42, 0.12)` — 단일 값 (dark/light 분리 명시 없음 → 두 모드 동일)

`apps/web/src/app/globals.css`는 `:root` 블록(L9-25)에 dark mode 기본값, `.light` 블록(L27-37)에 light mode 오버라이드를 정의한다. Tailwind v4 `@theme inline`(L40-61)이 이 토큰을 utility 클래스로 노출한다.

ThemeToggle 컴포넌트(`apps/web/src/components/ui/ThemeToggle.tsx`)는 `document.documentElement.classList.add("light")`를 통해 `.light` 클래스를 활성화한다. localStorage 키 `fugue-theme`로 상태 유지.

현 상태 검증:

| 토큰 | Dark mode (`:root`) | 명세 | Light mode (`.light`) | 명세 |
|------|------|------|------|------|
| `--text-muted` | `#888888` ✓ | `#888888` | `#666666` ✗ | `#777777` |
| `--text-dim` | `#555555` ✓ | `#555555` | `#999999` ✗ | `#AAAAAA` |
| `--accent-subtle` | `rgba(232, 90, 42, 0.12)` ✓ | `rgba(232, 90, 42, 0.12)` | `rgba(232, 90, 42, 0.08)` ✗ | `rgba(232, 90, 42, 0.12)` |
| `--border` | `#2A2A2A` ✓ | `#2A2A2A` | `#D0D0D0` ✗ | `#E0E0E0` |

Dark mode 토큰 15건은 모두 명세 일치, light mode 4건만 잔여 어긋남.

## Goals

- 코드 SSoT(globals.css)를 디자인 가이드 SSoT(DESIGN.md)와 정렬한다.
- light mode 토글이 디자인 가이드 의도대로 렌더링되도록 한다.
- 변경 폭을 globals.css `.light` 블록 4줄 값 교체로 한정한다.

## Non-Goals

- DESIGN.md 자체의 명세값 변경. (DESIGN.md는 SSoT이므로 코드가 따라야 함.)
- Dark mode 토큰 변경. (15건 모두 이미 일치.)
- 새로운 토큰 추가. (4건 모두 기존 토큰 값 교체.)
- 사용처(`text-text-muted`, `text-text-dim`, `border-border`, `bg-accent-subtle` 등 컴포넌트 className) 변경.
- light mode UI/UX 전반 점검. (본 변경은 토큰값 정렬에 한정.)

## Decisions

### Decision 1: globals.css `.light` 블록의 4줄을 직접 교체

DESIGN.md L41/L46/L48/L49 명세값을 globals.css `.light` 블록(L33-36)에 그대로 반영한다.

```css
.light {
  ...
  --text-muted: #777777;      /* was #666666, spec L48 */
  --text-dim: #AAAAAA;        /* was #999999, spec L49 */
  --accent-subtle: rgba(232, 90, 42, 0.12);  /* was 0.08, spec L41 */
  --border: #E0E0E0;          /* was #D0D0D0, spec L46 */
  ...
}
```

**대안 검토**:

1. **두 단계로 분리 처리** (text 토큰 / border·accent 토큰): 4건 모두 동일 파일·동일 블록의 단순 값 교체라 분리 이익 없음. 자체 리뷰 reject 조건 #4(effort 추정 차이) 위험 없음.
2. **DESIGN.md 명세 자체를 코드값에 맞춰 갱신**: DESIGN.md가 SSoT이고 코드가 따라야 함. 본 루프는 `apps/web/`만 본다(prompts/loop-design.md L7-8). DESIGN.md 변경은 사용자 결정 영역.
3. **사용처 className 일괄 점검 후 회귀 발견 시 사용처 변경**: 본 변경의 시각 폭은 17 step 이내(거의 동일 색상)이고 accent-subtle 0.08→0.12도 미세. 사용처 점검 없이도 안전. 회귀 발견 시 별도 후보로 분리.

→ 단일 커밋 4줄 직접 교체를 채택한다.

### Decision 2: 시각 회귀 검증은 dark mode 동등성 보장으로 한정

본 변경은 `.light` 블록만 수정하므로 dark mode(`:root` 블록)는 직접 무영향. 이 동등성은 git diff에서 `:root` 블록 미변경 + dark mode 토글 페이지 로드 시 시각 변경 없음으로 확인한다.

light mode 자체의 시각 변경은 의도된 SSoT 정렬이므로 회귀가 아님. 다만 명세값이 더 흐려지는 방향(text-muted #666→#777, text-dim #999→#AAA, border #D0→#E0)이라 가독성이 미세 감소할 수 있다. DESIGN.md L48-49 명세값이 그렇게 지정된 이상 코드가 따른다.

### Decision 3: anti-pattern 검토

- L15 (Tailwind 기본 클래스 의미 덮어쓰기): globals.css `--text-muted` 등은 Tailwind 기본 클래스가 아닌 커스텀 토큰. `text-text-muted`는 `--color-text-muted` → `--text-muted` 경로로 별도 토큰 네임스페이스. 미적용.
- L16 (DESIGN.md radius scale 자의적 매핑): 색 영역, 미적용.
- L17 (DESIGN.md 등급 매핑 미명시 자의적 해석): DESIGN.md L41/L46/L48/L49가 직접 명시값(`#777777` 등). 자의 없음. 미적용.
- L18 (apps/api 데이터 모델 동반): globals.css 단일 변경, 데이터 모델 무관. 미적용.
- L19 (EmptyState variant API 등 컴포넌트 API 확장): 토큰 값 교체, API 변경 없음. 미적용.
- L20 (breakpoint 토큰 덮어쓰기): 색 영역, 미적용.

## Risks

- **R1: light mode 사용자가 가독성 미세 감소를 보고**: text-muted/dim이 17 step씩 밝아져 어두운 글자가 미세하게 더 흐려짐. → DESIGN.md 명세값이 그렇게 지정. 사용자 보고가 있으면 DESIGN.md 갱신 절차로 분리(`apps/web/` 외).
- **R2: accent-subtle 0.08→0.12 50% 증가로 태그 배경이 두드러짐**: 작품과 경쟁하지 않는다는 Aesthetic Direction(L38)과 미세 텐션. → DESIGN.md가 0.12 단일 값을 명시함. 코드는 따른다.
- **R3: dark mode 회귀**: `:root` 블록 미수정으로 직접 무영향. git diff로 확인.
