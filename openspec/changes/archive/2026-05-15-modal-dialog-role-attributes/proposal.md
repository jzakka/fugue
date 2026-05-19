# 모달 2곳 패널에 `role="dialog"`/`aria-modal`/`aria-labelledby` 트리플 부여

## Backlog id

`design-20260515-modal-dialog-role-missing`

## 무엇을

VideoTrimModal과 AddToBoardButton 모달의 패널 컨테이너 div에 ARIA dialog 시맨틱 3속성(`role="dialog"`, `aria-modal="true"`, `aria-labelledby={titleId}`)을 추가하고, 짝꿍 `<h2>` 제목에 해당 id를 부여한다.

| 파일 | 패널 div 라인 | 제목 h2 라인 | titleId |
|------|------|------|------|
| `apps/web/src/components/pin/VideoTrimModal.tsx` | 121 | 123 | `video-trim-modal-title` |
| `apps/web/src/components/board/AddToBoardButton.tsx` | 201 | 207 | `add-to-board-modal-title` |

## 왜

### WAI-ARIA / WCAG 근거

- **WAI-ARIA Authoring Practices Guide — Dialog (Modal) Pattern** — 모달 dialog는 (1) `role="dialog"`로 컴포넌트 시맨틱 식별, (2) `aria-modal="true"`로 백그라운드가 비활성화되었음을 AT에 안내, (3) `aria-labelledby`로 dialog 제목과 연결의 3속성 트리플이 표준 구성.
- **WCAG 2.1 SC 4.1.2 Name, Role, Value (Level A)** — 모든 UI 컴포넌트는 (이름, 역할, 속성·상태·값)이 프로그래밍 방식으로 결정될 수 있어야 한다. 현재 두 모달은 역할(dialog)도 이름(연결된 제목)도 없다.

### 루프 정체성 근거

`prompts/loop-design.md` L7 — "apps/web/ 안의 ... 접근성(대비, 키보드 포커스, aria)" in-scope 명시.

### 현재 상태 측정

```bash
grep -rE 'role="dialog"|aria-modal|aria-labelledby' apps/web/src
# → 0건
```

두 모달의 패널 div는 dialog 시맨틱 없이 일반 `<div>`로 마크업되어 있어, 스크린리더는 모달 진입 시 "대화 상자, 비디오 구간 선택" 또는 "대화 상자, 보드에 추가"가 아닌 일반 region으로 처리한다. h2 제목은 페이지 일반 헤딩으로 읽혀 dialog 컨텍스트 정보가 누락된다.

### 사용자 영향

- 스크린리더 사용자(VoiceOver/NVDA/JAWS): 모달 진입 시 "대화 상자, 비디오 구간 선택" / "대화 상자, 보드에 추가"로 안내되어 모달이 열린 사실과 제목을 즉시 인지. `aria-modal="true"`로 백그라운드 콘텐츠는 외부 영역임이 명시되어 탐색 시 모달 안에 머무름이 유도된다.
- 시각 사용자: 변경 없음. ARIA 속성은 시각적 표현이 없는 의미 레이어 추가.

## 어디까지

### 변경 파일

- `apps/web/src/components/pin/VideoTrimModal.tsx` — L121 패널 div에 3속성, L123 h2에 `id` 1속성.
- `apps/web/src/components/board/AddToBoardButton.tsx` — L201 패널 div에 3속성, L207 h2에 `id` 1속성.

총 8개 속성 추가(3 ARIA × 2 모달 + 1 id × 2 h2).

### 시각/행동 변경

- 시각 변경 0. 모든 className/스타일/레이아웃 유지.
- 행동 변경 0(브라우저 동작 측면). ARIA 속성은 보조 기술 전용 시맨틱 레이어.
- AT 안내 변경: 모달 진입 시 dialog 시맨틱으로 안내.

### 무엇을 하지 않는가

- **ESC 키 처리 추가** — VideoTrimModal에는 ESC 핸들러가 없지만 본 후보 범위가 아니다(AddToBoardButton은 L124-131에 이미 ESC 핸들러 보유). 별도 후보로 분리.
- **Focus trap** — 모달 내부로 포커스를 가두는 동작 부재. WAI-ARIA Dialog Pattern의 keyboard interaction 항목이지만 구현 복잡도가 다르므로 별도 후보.
- **Initial focus 지정** — 모달 진입 시 어떤 요소에 포커스가 가는지 명시하는 작업. 별도 후보.
- **Document에 inert 적용** — 모달 외부 콘텐츠를 비활성화하는 `inert` 속성. 본 사이클은 dialog 시맨틱 트리플로 한정.

## 롤백

8개의 추가 속성을 제거. `git diff`로 명확히 revert 가능.

## Anti-pattern 검토

- **L15** (token 의미 덮어쓰기 vs 추가 분리): 해당 없음. ARIA 속성 추가만 수행.
- **L16** (radius 등급 매핑 모호): 해당 없음.
