# Proposal: create-board-cancel-busy-gating

## Why

비동기 폼의 취소 버튼 busy 게이팅이 표면 간 갈린다. role-identical 비동기 폼 취소 5곳 중 3곳(PinCreateForm·ProfileEditForm·BoardActions)은 busy 중 `disabled` + `disabled:opacity-50`로 게이팅하지만, 인라인 보드 생성 폼 2곳(MyPageClient·AddToBoardButton)의 취소 버튼만 `disabled` 속성 자체가 없다.

행위 결함: 보드 생성 요청이 in-flight인 동안(creating=true) 취소를 누르면 폼이 숨겨지고 입력이 초기화되지만 POST 요청은 계속 진행되어 보드가 그대로 생성된다 — 취소가 취소하지 못한다. 게이팅된 3곳은 이 창을 disabled로 봉쇄한다.

## What Changes

- MyPageClient 새 보드 폼의 취소 버튼에 `disabled={creating}` + `disabled:opacity-50` 추가
- AddToBoardButton 새 보드 폼의 취소 버튼에 `disabled={creating}` + `disabled:opacity-50` 추가

## Capabilities

### Modified: board

보드 생성 폼의 취소 버튼이 생성 요청 진행 중 비활성화되는 요구사항 추가.

## Impact

- `apps/web/src/components/profile/MyPageClient.tsx` — 취소 버튼 1곳 (2속성)
- `apps/web/src/components/board/AddToBoardButton.tsx` — 취소 버튼 1곳 (2속성)
- 로직·API·레이아웃 변경 없음. busy가 아닐 때의 취소 동작(폼 닫힘·입력 초기화)은 불변.
