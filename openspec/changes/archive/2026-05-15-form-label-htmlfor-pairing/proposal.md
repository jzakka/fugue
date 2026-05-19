# 폼 직접 페어 5곳에 `htmlFor`/`id` 라벨-컨트롤 연결 부여

## Backlog id

`design-20260515-form-label-htmlfor-missing`

## 무엇을

새 핀 만들기(PinCreateForm)와 프로필 편집(ProfileEditForm) 폼의 직접 label-control 페어 5곳에 `htmlFor`(label) + `id`(input/textarea) 속성을 추가해 라벨과 컨트롤을 프로그래밍 방식으로 연결한다.

| 파일 | 라인 | label 텍스트 | 부여할 id |
|------|------|------|------|
| `apps/web/src/app/pin/new/PinCreateForm.tsx` | 435-446 | 제목 * | `pin-title` |
| `apps/web/src/app/pin/new/PinCreateForm.tsx` | 450-458 | 설명 | `pin-description` |
| `apps/web/src/app/pin/new/PinCreateForm.tsx` | 462-471 | 원본 URL (선택) | `pin-url` |
| `apps/web/src/components/profile/ProfileEditForm.tsx` | 62-69 | 닉네임 | `profile-nickname` |
| `apps/web/src/components/profile/ProfileEditForm.tsx` | 74-83 | 아바타 URL | `profile-avatar-url` |

## 왜

### WCAG 근거

- **WCAG 2.1 SC 1.3.1 Info and Relationships (Level A)** — 콘텐츠의 정보, 구조, 관계는 프로그래밍 방식으로 결정될 수 있어야 한다. 라벨과 컨트롤의 관계는 `<label for>` 또는 implicit nesting으로 표현되어야 한다.
- **WCAG 2.1 SC 4.1.2 Name, Role, Value (Level A)** — 모든 UI 컴포넌트는 접근 가능한 이름을 가져야 한다. 폼 컨트롤에서 라벨은 접근 이름의 1차 소스.

### 루프 정체성 근거

`prompts/loop-design.md` L7 — "apps/web/ 안의 ... 접근성(대비, 키보드 포커스, aria)" in-scope 명시.

### 현재 상태 측정

5곳 모두 sibling 구조이면서 매핑 속성 부재:

```tsx
// 현재 (5곳 동일 패턴)
<div>
  <label className="block text-sm text-text-muted mb-2">제목 *</label>
  <input type="text" value={title} ... />
</div>
```

`grep "htmlFor" apps/web/src/app/pin/new/PinCreateForm.tsx apps/web/src/components/profile/ProfileEditForm.tsx` → 0건.

### 사용자 영향

- 마우스 사용자: 라벨 텍스트("제목"/"설명"/"닉네임" 등)를 클릭하면 짝꿍 input으로 포커스 이동. 현재는 클릭이 무반응.
- 스크린리더 사용자(VoiceOver/NVDA/JAWS): input에 Tab 도달 시 라벨 텍스트가 명시적 접근 이름으로 안내된다. 현재는 일부 AT의 추론에 의존하며 표준 보장이 없어 placeholder 또는 type만 읽히는 경우 발생.
- 음성 제어(Voice Control): "Click 제목" 같은 명령으로 input 활성화 가능해진다.

## 어디까지

### 변경 파일

- `apps/web/src/app/pin/new/PinCreateForm.tsx` (3쌍, L435/438, L450/451, L462/465)
- `apps/web/src/components/profile/ProfileEditForm.tsx` (2쌍, L62/63, L74/77)

각 라벨에 `htmlFor=`, 각 컨트롤에 `id=` 한 속성씩 추가. 총 10개 속성.

### ID 명명 규칙

- 폼 단위 prefix + 필드 의미: `pin-{title,description,url}`, `profile-{nickname,avatar-url}`
- 페이지 내 다른 컴포넌트와 충돌 가능성: 새 핀 만들기 페이지는 폼 하나만, 프로필 편집은 모달성 폼이라 동일 페이지에 동일 ID가 두 번 렌더되지 않음 → 정적 ID로 충분(useId 불필요).

### 사용자 영향

- 시각 변경 0. 모든 className/스타일 유지.
- 행동 변경: 라벨 클릭 시 input 포커스 발생(브라우저 기본 동작).
- AT 안내 변경: 접근 이름 보장.

### 무엇을 하지 않는가

- **그룹 라벨 3곳** (`PinCreateForm.tsx:326` 미디어 파일, `PinCreateForm.tsx:505` 태그, `VideoThumbnailPicker.tsx:127` 썸네일 선택)은 단일 컨트롤이 아닌 위젯 그룹의 라벨이라 `fieldset/legend` 또는 `role="radiogroup"/aria-labelledby` 패턴이 적절. 본 후보에서 제외하고 별도 후보로 분리.
- **필수 표시 (`<span className="text-error">*</span>`)** 의 `aria-required` 동기화는 별도 a11y 후보로 분리.
- **에러 메시지 ↔ 컨트롤 `aria-describedby` 연결**은 PinCreateForm/ProfileEditForm 모두 폼 상단에 통합 에러 박스를 두는 패턴이라 필드별 에러가 없어 본 사이클 범위 아님.

## 롤백

5쌍의 추가한 `htmlFor`/`id` 속성을 제거. `git diff`로 깔끔하게 revert 가능.

## Anti-pattern 검토

- **L15** (token 의미 덮어쓰기 vs 추가 분리): 해당 없음. Tailwind 토큰과 무관.
- **L16** (radius 등급 매핑 모호): 해당 없음.
