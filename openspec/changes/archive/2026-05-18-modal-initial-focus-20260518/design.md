## Context

플랫폼의 modal dialog는 현재 두 컴포넌트로 정착되어 있다:
- `VideoTrimModal`(`apps/web/src/components/pin/VideoTrimModal.tsx`) — 핀 생성 시 비디오 트림 dialog
- `BoardSelectModal`(`apps/web/src/components/board/AddToBoardButton.tsx` 내부) — 핀을 보드에 추가하는 dialog

두 modal 모두 누적 사이클로 다음 패턴을 받았다:
- 사이클 25: `role="dialog" aria-modal="true" aria-labelledby` 트리플
- 사이클 56: Escape 키 닫기 + body scroll lock
- 사이클 70: overlay click 닫기(VideoTrimModal — AddToBoardButton은 이미 정착)

남은 잔여: Initial focus(modal 열린 직후 dialog 내부로 포커스 이동) · Focus trap(Tab 키 modal 외부 이탈 차단) · 닫힌 후 trigger로 focus 복귀.

본 사이클은 그중 단발 처리 가능한 **Initial focus**만 정렬한다. WAI-ARIA Authoring Practices Guide Dialog Pattern은 "When a dialog opens, focus moves to an element contained in the dialog. Where focus moves depends on the nature of the dialog"를 명시. 일반 규칙은 가장 우선순위 높은 actionable 요소 또는 dialog container 자체에 focus를 둔다.

## Goals / Non-Goals

**Goals:**
- 두 modal이 열린 직후 키보드 포커스가 dialog 내부에 위치하도록 보장.
- 키보드 사용자가 Escape 즉시 누름 / Tab으로 컨트롤 순회 가능한 시작 상태 확립.

**Non-Goals:**
- Focus trap (Tab/Shift-Tab을 modal 내부로 가두는 패턴) — 별도 후보로 보류(effort 4, risk 3, confidence 2로 단발 사이클 패턴 초과).
- 닫힌 후 trigger 요소로 focus 복귀 — 별도 후보로 보류.
- 첫 번째 actionable 요소 자동 포커스(예: AddToBoardButton의 첫 board 버튼) — panel container 포커스가 더 보수적이며 모든 modal 본문 구조에 일관 적용 가능. WAI-ARIA Pattern도 dialog container에 focus를 두는 변형을 허용.
- AddToBoardButton L316 조건부 "새 보드 생성" form input `autoFocus` 패턴(기존 동작 유지).

## Decisions

### Decision 1 — `tabIndex={-1}` + `panelRef.current?.focus()` 패턴
두 modal panel div에 `tabIndex={-1}` 추가하고 본문에 `useEffect(() => { panelRef.current?.focus(); }, []);` 한 블록 추가.

`tabIndex={-1}` 의미:
- div는 기본적으로 focusable하지 않음. `tabIndex={-1}`은 프로그래밍 방식 `.focus()` 호출은 받지만 자연스러운 Tab 순회에는 포함되지 않는 상태.
- 즉 modal 열리는 시점에는 programmatic focus를 받지만 Tab 키로는 dialog container를 다시 만날 일이 없어 사용자 Tab 순회를 방해하지 않음.

**대안:**
- 첫 actionable 요소(예: VideoTrimModal의 "취소" 버튼, AddToBoardButton의 닫기 버튼)에 ref + autofocus — 두 modal의 본문 구조가 달라 ref 분기·우선순위 매핑이 자의적이 될 수 있음. WAI-ARIA Pattern이 dialog container focus 변형을 허용하므로 두 modal 일관 적용 가능한 panel focus 선택.
- `autoFocus` 속성 사용 — React `autoFocus`는 mount 시 한 번만 동작하며 ref-less 패턴이지만, panel div(`<div>`)에는 자연스럽지 않고 dialog 컨테이너에 명시적 `.focus()` 호출이 의도를 더 명확히 표현. 기존 `panelRef`를 이미 활용 중이라 일관성 우위.

### Decision 2 — Mount 시점 1회 호출, deps `[]`
`useEffect(() => { panelRef.current?.focus(); }, []);` 한 블록을 추가. modal mount 직후 panel에 focus 이동, cleanup 없음.

두 modal 모두 mount 시 conditional render(예: AddToBoardButton `isOpen && <BoardSelectModal>`, PinCreateForm `showTrim && <VideoTrimModal>`)로 열림이 BoardSelectModal/VideoTrimModal 컴포넌트 mount와 동치. 따라서 `[]` deps로 mount 시점 1회 호출이 정확한 시점에 동작.

**대안:**
- ref callback에 focus 호출 — ref callback은 mount 외에도 re-render 시 호출될 수 있어 useEffect [] 패턴이 더 명시적.
- 외부 trigger prop watching — 두 modal 모두 자체 mount/unmount로 열림이 표현되므로 불필요.

### Decision 3 — AddToBoardButton L316 `autoFocus` 충돌 점검
`AddToBoardButton.tsx:316`의 input `autoFocus`는 "새 보드 생성" form이 펼쳐졌을 때만 렌더되는 conditional input의 mount 시점 focus. modal 자체가 열린 직후에는 `showCreate=false`라 input이 렌더되지 않음 → modal mount 시점에는 panel focus가 우선, 사용자가 "+ 새 보드 만들기" 버튼 누르면 input mount 시 `autoFocus`가 정상 동작.

따라서 두 focus 시점은 시간상 분리되어 충돌하지 않음. 검증 task로 명시.

**대안:**
- 기존 autoFocus 제거하고 ref 기반 통합 — 본 사이클 범위 초과. 기존 패턴 유지.

## Risks / Trade-offs

- **[리스크] 환경 제약으로 dev 서버 시각 검증 미수행** → `node_modules` 미설치. 코드 검증(grep, diff)으로 대체. 패턴이 표준 React + WAI-ARIA Pattern 1:1 매핑이라 동작 회귀 위험 낮음.
- **[리스크] modal 외부 trigger 버튼에서 modal 닫힌 직후 focus가 trigger로 복귀하지 않음** → 본 사이클 Non-Goals. 별도 후보로 분리 보류.
- **[리스크] `panelRef.current`가 첫 render mount 직후 null 가능성** → React useEffect는 commit 단계(DOM 부착 후)에 실행되므로 ref는 이미 할당된 상태. `panelRef.current?.focus()` optional chaining으로 null-safe.
- **[Trade-off] panel container focus는 첫 actionable 요소 focus보다 보수적** → "취소 버튼이 강조된" 느낌 없음. 대신 컨트롤 그룹에 진입한 사용자 의도가 명확하지 않을 때 panel focus가 더 안전. WAI-ARIA Pattern이 두 변형 모두 허용.
- **[리스크] AddToBoardButton에서 modal panel focus 후 사용자가 "새 보드 생성" 펼침 → input `autoFocus` 동작 확인 필요** → modal mount 시점 panel focus와 conditional input mount 시점 `autoFocus`는 시간상 분리. 검증 task에 명시.

## Migration Plan

1. 두 파일 각각 2단계 편집.
2. git commit 1건.
3. 롤백: `git revert <sha>` 1회.
