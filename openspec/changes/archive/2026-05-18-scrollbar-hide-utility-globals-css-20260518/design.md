## Context

`apps/web/`의 5곳 가로 스크롤 영역(`overflow-x-auto`)에 `scrollbar-hide` className이 부여되어 있다. 작성자 의도는 OS 기본 가로 스크롤바를 숨기는 것이지만, 이 클래스는 어디에도 정의되어 있지 않다.

3중 점검 결과:
1. `apps/web/src/app/globals.css` — `.scrollbar-hide` 정의 없음 (`grep`으로 직접 확인).
2. `apps/web/postcss.config.mjs` — `@tailwindcss/postcss` 단일 플러그인 등록, scrollbar 관련 플러그인 없음.
3. `apps/web/package.json` — dependencies/devDependencies에 `tailwindcss-scrollbar-hide` 등 scrollbar 관련 패키지 없음. Tailwind v4 코어 utilities에는 `scrollbar-*`이 포함되지 않는다(v3에서도 마찬가지).

결과: 5곳에서 OS 기본 가로 스크롤바가 그대로 노출되어 DESIGN.md L11 "Decoration level: Minimal" 명세 위반.

이전 직접 관련된 결정 이력은 없다 (`.fugue/decision-log.md` 기준).

## Goals / Non-Goals

**Goals:**
- 5곳 가로 스크롤 영역의 OS 기본 가로 스크롤바를 일관되게 숨긴다.
- 스크롤 동작 자체(터치 스와이프, 휠 가로 스크롤, 키보드 화살표)는 그대로 유지한다.
- 사용처 5곳의 className·컴포넌트 코드는 변경하지 않는다. 단일 진입점(`globals.css`)에 utility 정의만 추가한다.

**Non-Goals:**
- 세로 스크롤바 숨김은 본 변경 범위 밖.
- `@theme inline` 블록 안의 Tailwind 토큰 시스템에 손대지 않는다.
- 사용처 5곳의 padding·gap·기타 spacing 미세조정은 본 변경 범위 밖.
- 다른 의도 미반영 영역(예: 세로 스크롤이 있는 패널) 일괄 적용은 본 변경 범위 밖.

## Decisions

### Decision 1: Plain CSS rule로 정의 (Tailwind `@utility` 디렉티브 미사용)

`globals.css` 끝부분에 plain CSS 룰로 정의한다:

```css
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}
```

**대안 1 — Tailwind v4 `@utility` 디렉티브 사용:**
```css
@utility scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
  &::-webkit-scrollbar { display: none; }
}
```
- 장점: variants(`hover:scrollbar-hide` 등)와 통합 가능.
- 단점: 사용처 5곳 모두 variant 없는 plain `scrollbar-hide`만 사용. variants 통합 요구 없음. `globals.css` 기존 패턴(`.skeleton-shimmer .bg-surface-elevated`, `.font-mono`, `body` 등)이 모두 plain CSS rule을 사용하므로 일관성 측면에서 plain rule이 적합.

**대안 2 — `tailwindcss-scrollbar-hide` 플러그인 도입:**
- 장점: 표준 패키지.
- 단점: 단일 utility를 위한 dependency 추가는 과잉. `globals.css`에 3줄로 해결 가능.

**선택**: Plain CSS rule. `globals.css`의 기존 plain CSS rule 패턴(`.font-mono { font-variant-numeric: tabular-nums; }`, `.skeleton-shimmer .bg-surface-elevated { ... }`)과 동일 스타일.

### Decision 2: 3개 CSS property 사용 (`-ms-overflow-style` + `scrollbar-width` + `::-webkit-scrollbar`)

각 브라우저 엔진별 스크롤바 숨김 표준이 분리되어 있다:
- `-ms-overflow-style: none` — 구 Edge·IE.
- `scrollbar-width: none` — Firefox, Chromium(121+) 표준.
- `::-webkit-scrollbar { display: none }` — Webkit·Blink 계열(Safari, 구 Chromium).

세 가지 모두 합쳐야 모든 주요 브라우저에서 일관되게 스크롤바가 숨겨진다. `scrollbar-width`가 Webkit 진영에서도 점차 지원되지만 안전망으로 `::-webkit-scrollbar`도 함께 둔다 (사용처가 모바일·데스크톱 모두 포함).

### Decision 3: `globals.css` 위치는 `@theme inline` 블록 밖, 기존 plain CSS rule 그룹 안

```
@theme inline { ... }              ← 토큰 정의
.font-mono { ... }                  ← 기존 plain CSS rule
body { ... }
.masonry-grid { ... }
.masonry-grid_column { ... }
.masonry-grid_column > * { ... }
@keyframes shimmer { ... }
.skeleton-shimmer .bg-surface-elevated { ... }
.scrollbar-hide { ... }             ← 본 변경 — 파일 끝부분 append
.scrollbar-hide::-webkit-scrollbar { ... }
```

기존 plain CSS rule 그룹과 같은 위치(`@theme inline` 밖)에 두어 토큰 시스템과 의미 분리를 명확히 한다.

### Decision 4: 사용처 5곳 코드 미변경

`scrollbar-hide` className은 이미 5곳에 부여되어 있으므로, `globals.css`에 정의가 추가되는 순간 자동으로 효과가 발생한다. 사용처 코드를 변경하지 않는다.

## Risks / Trade-offs

- **[Risk] Tailwind v4 코어에 `scrollbar-hide`가 실제로 포함되어 있어서 신규 정의가 기본을 덮어쓰게 됨** → Mitigation: `apps/web/package.json` dependencies + `apps/web/postcss.config.mjs` + `apps/web/src/app/globals.css` 3중 점검으로 미정의 강하게 확인됨. 추가로 Tailwind v3/v4 표준 utilities에는 `scrollbar-*`이 코어로 포함된 적 없음(공식 docs 기준). 본 변경이 신규 클래스 정의이지 기본 의미 덮어쓰기가 아니므로 anti-pattern L15 미해당.
- **[Risk] 스크롤바가 숨겨져 사용자가 가로 스크롤 가능 여부를 인지 못함** → Mitigation: 5곳 사용처 모두 `overflow-x-auto`로 가로 스크롤이 실제로 가능하지만, 현재 코드가 이미 `scrollbar-hide` className으로 스크롤바 숨김을 의도하고 있어 본 변경은 작성자 의도를 실제 동작에 정합시키는 것일 뿐이다. 사용자 인지 측면 변경은 본 변경의 추가 결정이 아님. 또한 5곳 모두 가로로 짧은 필터/탭 UI라 스크롤 가능 여부가 시각적으로 자명(아이템이 우측에서 잘림).
- **[Trade-off] `::-webkit-scrollbar` 셀렉터는 비표준 가상요소** → 모든 주요 브라우저(Safari·Chrome·Edge)에서 동작이 안정적이며, 표준화 진행 중. 사용 시 일반적 패턴(MDN 권장 패턴 일치).
- **[Trade-off] 가로 스크롤바 없이는 마우스 휠 가로 스크롤이 일부 OS 환경에서 어려울 수 있음** → 5곳 사용처는 모바일 우선 UI 패턴(필터 칩/탭)이라 터치 스와이프가 주요 인터랙션. 데스크톱에서도 시프트+휠 또는 트랙패드로 가로 스크롤 가능.

## Migration Plan

- **배포**: 단일 커밋. CSS만 변경되므로 빌드 변경 없이 즉시 반영.
- **롤백**: `git revert <commit>` 으로 단일 커밋 되돌리기.
- **시각 검증**: 5곳 사용처를 dev 서버에서 띄워 가로 스크롤바 비노출 + 가로 스크롤 동작 유지 확인 (본 변경 범위 안에서는 수동 시각 검증).
