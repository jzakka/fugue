## Why

`apps/web/src/components/pin/VideoTrimModal.tsx`와 `apps/web/src/components/board/AddToBoardButton.tsx`의 `BoardSelectModal`은 둘 다 `role="dialog" aria-modal="true"` modal dialog인데 열린 직후 panel에 포커스를 이동시키는 Initial focus 패턴이 없다. 키보드 사용자는 modal이 열린 직후 Tab을 누르면 modal 바깥 요소(NavBar 링크·뒤의 폼 컨트롤)로 포커스가 이동할 수 있다. WAI-ARIA Authoring Practices Guide Dialog (Modal) Pattern은 "When a dialog opens, focus moves to an element contained in the dialog"를 명시한다. cycle 56 archive(`2026-05-15-video-trim-modal-scroll-lock`)·cycle 70 archive(`2026-05-18-video-trim-modal-overlay-click-close-20260518`)의 decision-log note가 "Initial focus·Focus trap은 별도 후보로 분리 유지"라고 본 후보를 명시적으로 예약했다.

## What Changes

- 두 modal panel div에 `tabIndex={-1}` 추가:
  - `apps/web/src/components/pin/VideoTrimModal.tsx` 내부 dialog div (L147-152).
  - `apps/web/src/components/board/AddToBoardButton.tsx` panel div (L201-207).
- 두 modal 본문에 `useEffect(() => { panelRef.current?.focus(); }, []);` 한 블록 추가(deps `[]`). 두 modal 모두 cycle 70 / 기존 `panelRef`(`useRef<HTMLDivElement>(null)`) 활용.

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `design-tokens`: 디자인 시스템 일관성 capability에 모달 dialog Initial focus 표준 요구사항 추가(디자인 트랙 archive 누적 패턴 유지).

## Impact

- 변경 파일: 2개 (`VideoTrimModal.tsx`, `AddToBoardButton.tsx`).
- 변경 범위: 파일당 2단계(panel div `tabIndex={-1}` + Initial focus `useEffect`) × 2 파일 = 4단계.
- 사용자 영향: 모달이 열린 직후 panel에 포커스가 이동 → Tab 키 흐름이 modal 내부로 시작, Escape 키도 즉시 동작. 키보드 사용자 흐름이 modal과 분리되지 않음.
- 비포함: Focus trap(Tab/Shift-Tab을 modal 내부로 가두는 패턴), 닫힌 후 trigger 요소로 focus 복귀, AddToBoardButton L316 조건부 form `autoFocus` 패턴(기존 유지).
- 롤백: git revert 단일 커밋.
