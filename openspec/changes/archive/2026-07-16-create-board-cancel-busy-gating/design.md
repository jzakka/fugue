# Design: create-board-cancel-busy-gating

## Context

비동기 폼 취소 버튼 census (5곳, VideoTrimModal은 동기 선택 모달이라 모집단 제외):

| 표면 | busy state | 취소 disabled | 디밍 |
|------|-----------|--------------|------|
| PinCreateForm:654 | isDisabled | `disabled={isDisabled}` | `disabled:opacity-50` |
| ProfileEditForm:118 | saving | `disabled={saving}` | `disabled:opacity-50` |
| BoardActions:109 | saving | `disabled={saving}` | `disabled:opacity-50` |
| MyPageClient:126 | creating | **없음** | **없음** |
| AddToBoardButton:389 | creating | **없음** | **없음** |

BoardActions는 outlier 2곳과 동일한 컴팩트 인라인 보드 폼(px-3 py-1.5 text-xs)인데 게이팅하므로 컴팩트/대형 컨텍스트 상관으로 설명 불가 — 동일 클래스 내 단독 이탈.

## Goals

- 지배 관례(3/5)인 "busy 중 취소 disabled + opacity 디밍"으로 outlier 2곳 정렬
- busy가 아닐 때의 취소 동작·레이아웃·다른 상태 로직 불변

## Non-Goals

- in-flight 요청의 실제 abort(AbortController) 도입 — 게이팅 관례 정렬 범위 밖
- VideoTrimModal 취소(동기 모달, busy 부재)

## Decisions

### Decision 1: `disabled={creating}` 속성 추가 (상태 재사용)

두 컴포넌트 모두 이미 `creating` state를 보유(MyPageClient:16, AddToBoardButton:103)하고 짝꿍 생성 버튼이 `disabled={creating}`을 사용 중이다. 취소 버튼에 동일 상태를 그대로 바인딩한다. 새 상태·prop 도입 없음.

대안 기각: AbortController로 실제 취소 구현 — 관례 정렬(게이팅) 범위를 벗어난 기능 추가이며, 게이팅 3곳도 abort는 하지 않는다.

### Decision 2: className에 `disabled:opacity-50` 추가

backlog design-20260515-cancel-button-disabled-style-missing(done) 선례가 "disabled 취소 버튼은 `disabled:opacity-50` 디밍 동반"을 확립했다. disabled 속성을 새로 받는 두 버튼에 동일 클래스를 추가해 시각 채널도 관례 정렬한다.

MyPageClient 취소 (before):
```tsx
<button
  onClick={() => { setShowCreate(false); setNewBoardName(""); setError(null); }}
  className="px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors cursor-pointer"
>
```

after:
```tsx
<button
  onClick={() => { setShowCreate(false); setNewBoardName(""); setError(null); }}
  disabled={creating}
  className="px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors cursor-pointer disabled:opacity-50"
>
```

AddToBoardButton 취소도 동일 패턴(`setError` 없음만 다름).

### Decision 3: `aria-busy`는 추가하지 않음

기존 게이팅 3곳의 취소 버튼도 `aria-busy`를 달지 않는다(aria-busy는 작업을 수행하는 버튼에만). 관례 그대로 따른다.

## Risks

- 낮음: creating은 요청 완료 시 항상 false로 복귀(성공·실패 공통 finally/후속 setState)하므로 취소가 영구 잠기지 않음. 확인: MyPageClient handleCreate·AddToBoardButton handleCreateAndAdd 양쪽 실패 경로에서 setCreating(false) 호출 여부를 구현 시 재검증.
