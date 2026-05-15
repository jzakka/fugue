# 취소 버튼 3곳에 `disabled:opacity-50` 추가

## Backlog id

`design-20260515-cancel-button-disabled-style-missing`

## 무엇을

세 폼의 취소(Cancel) 버튼 className 끝에 `disabled:opacity-50` 토큰을 추가한다. 짝꿍 submit/저장 버튼은 모두 이미 이 클래스를 갖고 있어 비대칭 상태가 해소된다.

| 파일 | 라인 | 동작 |
|------|------|------|
| `apps/web/src/app/pin/new/PinCreateForm.tsx` | 601 | className 끝에 ` disabled:opacity-50` 추가 |
| `apps/web/src/app/boards/[id]/BoardActions.tsx` | 99 | 동일 |
| `apps/web/src/components/profile/ProfileEditForm.tsx` | 104 | 동일 |

## 왜

### 패턴 일관성 증거

코드베이스의 `<button disabled={...}>` 사용처는 총 16건. 그 중 13건은 className에 `disabled:opacity-50`(또는 `disabled:opacity-40` 1건)이 같이 있어 disabled 상태가 시각적으로 표현됨. 나머지 3건이 본 변경 대상 취소 버튼:

- `PinCreateForm.tsx:600-601` — `disabled={isDisabled}`인데 className에 disabled 토큰 없음. 짝꿍 submit `L607-608`은 `disabled:opacity-50` 보유.
- `BoardActions.tsx:98-99` — `disabled={saving}`인데 className에 disabled 토큰 없음. 짝꿍 저장 `L86-87`은 `disabled:opacity-50` 보유.
- `ProfileEditForm.tsx:103-104` — `disabled={saving}`인데 className에 disabled 토큰 없음. 짝꿍 저장 `L110-111`은 `disabled:opacity-50` 보유.

세 화면 모두 submit 누르면 saving=true로 바뀌어 짝꿍 취소도 자동으로 disabled가 되지만, 시각만 평소와 동일해 사용자가 누르려고 시도해도 무반응이다. 짝꿍 submit은 페이드되어 비활성 상태가 명시되므로 같은 행에 두 버튼의 disabled 표현이 비대칭이 된다.

### DESIGN.md 근거

DESIGN.md는 disabled 상태의 구체적 opacity 값을 직접 명시하지 않는다. 본 후보의 근거는 다음 두 축:

- **루프 정체성** (`prompts/loop-design.md` L7) — "디자인 시스템 일관성" in-scope. 코드베이스 패턴 측정 결과 disabled 토큰이 13/16 = 81% 일관 사용 중. 3건의 누락은 명백한 outlier.
- **DESIGN.md L88-94 Motion 섹션** — "Minimal-functional — 작품 감상을 방해하지 않는 선" + "Card hover: translateY(-2px), 200ms ease" 와 같이 상태 전이 시 시각 신호가 일관되어야 한다는 원칙. disabled도 상태 전이의 한 형태.

### 접근성

`<button disabled>`는 브라우저가 클릭을 차단하지만 시각적으로는 같은 톤이 유지되면 a11y 사용자(특히 색약/저시력)는 비활성 상태를 알아채기 어렵다. 짝꿍 submit이 페이드되는데 취소만 안 페이드되는 시각 비대칭은 학습된 패턴(둘 다 페이드 == 폼이 in-flight 중)을 깬다.

## 어디까지

### 변경 파일

- `apps/web/src/app/pin/new/PinCreateForm.tsx` (L601)
- `apps/web/src/app/boards/[id]/BoardActions.tsx` (L99)
- `apps/web/src/components/profile/ProfileEditForm.tsx` (L104)

각 파일 1줄, 토큰 1개씩 추가.

### 사용자 영향

폼 저장 in-flight 중(saving=true) 취소 버튼이 짝꿍 submit과 동일하게 페이드된다. 평상시(disabled=false) 시각은 전혀 변하지 않는다. 클릭 동작 변화 없음(이미 `disabled` prop으로 차단 중).

### 무엇을 하지 않는가

- `VideoTrimModal.tsx:212`, `AddToBoardButton.tsx:335`, `MyPageClient.tsx:113` 등 취소 버튼 — 이들은 애초에 `disabled` prop이 없어 시각 비대칭이 발생하지 않음. 손대지 않음.
- `disabled:cursor-not-allowed` 일관성 — 코드베이스에 2건만 있고 11건은 누락. 별도 후보로 분리.
- `disabled:opacity-40 vs disabled:opacity-50` 일관성 — `PinCreateForm.tsx:579` 태그 칩 1건만 opacity-40. 별도 후보로 분리.

## 롤백

세 파일에서 추가한 ` disabled:opacity-50` 토큰을 제거. `git diff`로 깔끔하게 revert 가능.

## Anti-pattern 검토

- **L15** (token 의미 덮어쓰기 vs 추가 분리): 해당 없음. Tailwind 기본 `disabled:opacity-50` 사용, 신규 토큰 정의 아님.
- **L16** (radius 등급 매핑 모호): 해당 없음.
