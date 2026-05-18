## Context

DESIGN.md L18-19는 두 카테고리를 분리해 weight를 다르게 명시한다.
- L18 'Body: Pretendard Variable'(weight 미명시 → 기본 400)
- L19 'UI/Labels: Pretendard Variable 500'(weight 500)

사이클 21(archive/2026-05-15-token-display-font-family)에서 display 카테고리를 `--font-display` 토큰으로 분리하고 inline style 사용처를 `font-display` 클래스로 정리했다. 사이클 23(archive/2026-05-15-token-mono-font-family)에서 data/tags 카테고리에 대해 동일 작업을 했다. 본 변경은 같은 패턴의 잔여 갭(labels 카테고리)을 닫는다.

현재 `<label>` HTML 요소 8곳이 모두 `className="block text-sm text-text-muted mb-2"`로 weight 미명시(grep 결과 0건). 결과적으로 body 기본 weight(400)로 렌더링되어 라벨/본문 위계가 동급.

## Goals / Non-Goals

**Goals:**
- DESIGN.md L19 명시 weight 500을 `<label>` HTML 요소에 적용한다.
- 사이클 21/23이 정착시킨 'DESIGN.md 카테고리 weight를 코드 SSoT에 반영' 패턴의 일관성을 유지한다.
- 변경 라인 수를 최소화(파일 3개, 8라인, 단어 1개씩 추가)한다.

**Non-Goals:**
- `<label>` 외의 광의 'UI labels' 해석(aria-label·메뉴 라벨·버튼 텍스트 등)에는 적용하지 않는다(매핑 자의성 회피).
- 신규 토큰 추가/덮어쓰기는 하지 않는다(`font-medium`은 Tailwind v4 기본 클래스로 정확히 weight 500을 의미).
- 라벨 색·크기·간격(`text-sm`/`text-text-muted`/`mb-2`)은 그대로 유지한다.

## Decisions

### `font-medium` 단어 추가 vs 토큰 정의 + 유틸리티

**선택**: `font-medium` 단어 추가.

**대안**:
- (A) globals.css `@theme inline`에 `--font-label-weight: 500;` 같은 신규 변수 도입 + 별도 유틸리티 클래스(예: `font-label`) 생성. → Tailwind v4가 자동 생성하는 `font-medium`이 이미 weight 500을 정확히 매핑하므로 추가 추상화가 불필요. anti-pattern L15 '기존 Tailwind 기본 클래스 의미 덮어쓰기'에는 해당 안 됨(기본 의미 = weight 500 = 의도와 일치).
- (B) `<label>` className에 `font-medium` 단어 1개 추가. → 변경 라인 수 최소, 시각 결과 동일, 회귀 위험 최소.

**근거**: 사이클 21·23은 토큰 도입이 필요했음(`General Sans`/`Geist Mono` 같은 비-기본 폰트 패밀리라 `@theme inline` 정의 필수). 본 변경은 weight 500을 표현하는 Tailwind 기본 클래스가 이미 존재하므로 추가 토큰 정의 없이 단순 클래스 추가만으로 동일 결과 달성.

### 적용 범위 한정 — `<label>` HTML 요소만

**선택**: `<label>` HTML 요소에만 적용. aria-label·메뉴 라벨·버튼 텍스트·태그 칩 텍스트는 본 변경 범위 밖.

**대안**:
- (A) DESIGN.md L19 'UI/Labels'의 광의 해석(폼 라벨 + 메뉴 라벨 + 칩 텍스트 + 버튼 라벨 등) → 매핑이 자의적(태그 칩이 'label'인가 'data'인가? 버튼 텍스트가 'label'인가 'body'인가?). anti-pattern L16 '등급 매핑이 DESIGN.md에 명시되지 않는 컴포넌트는 자의적 해석'에 부합.
- (B) `<label>` HTML 요소만 — 폼 컨트롤 라벨링이 'UI label'의 표준 정의이자 가장 명확한 1순위 매핑.

**근거**: 모호한 매핑은 자체 리뷰에서 reject 사유(#2 자의적 해석)에 걸린다. 명확한 매핑 1건으로 한정하고, 잔여 광의 'labels'는 별도 후보로 분리해 차후 사이클에서 카테고리별 결정.

## Risks / Trade-offs

- **시각 회귀 — 라벨 stroke 굵어짐**: weight 400 → 500은 약 100 단계 차이로 동일 글자에서 stroke가 미세하게 굵어진다. Pretendard Variable은 가변 폰트라 단계 사이 보간이 매끄럽고, 라벨 영역은 본문/입력값과 시각적으로 분리되도록 의도된 위계라 강조가 의도와 일치. → **완화**: 8개 라벨이 동시에 일관 적용되어 카테고리 내부 일관성 유지. PinCreateForm·VideoThumbnailPicker·ProfileEditForm 3개 폼에서만 노출되며 핀 카드/피드 등 핵심 콘텐츠 영역에는 영향 없음.
- **`font-medium`의 의미 폭주 가능성**: Tailwind 기본 `font-medium`은 모든 곳에서 weight 500을 의미. 본 변경 후 다른 컴포넌트가 무의도하게 `font-medium`을 상속하지 않음(클래스 추가는 명시적 적용처에만 한정). → **완화**: 변경은 className 추가뿐이며 globals.css·base styles에 영향 없음. 다른 컴포넌트는 영향 받지 않는다.
- **anti-pattern L15 적용 여부 점검**: `font-medium`은 Tailwind v4 기본 클래스로 weight 500을 의미. 본 변경은 신규 추가만 하고 globals.css에서 기본 의미를 덮어쓰지 않는다. → anti-pattern L15 적용 대상 아님.
- **anti-pattern L16 적용 여부 점검**: `<label>` HTML 요소는 'UI labels' 카테고리의 1순위 직접 대응. 자의적 등급 부여 아님. → anti-pattern L16 적용 대상 아님.

## Migration Plan

- 단일 커밋으로 적용. 의존성·환경변수·DB 변경 없음.
- 롤백: `git revert <commit>`.
- 사후 검증: `grep -rE '<label\\b[^>]*className=' apps/web/src --include='*.tsx' | grep -vE 'font-medium|font-semibold|font-bold'` 결과 0건 확인.

## Open Questions

- 광의 'UI labels' 카테고리(메뉴 라벨·버튼 텍스트·칩 텍스트)에 weight 500을 적용할지 여부는 본 변경 범위 밖. 카테고리별 매핑이 DESIGN.md에 명시되지 않아 각각 별도 사이클의 발견 후보로 분리 검토 필요.
