# Design: board-edit-enter-submit

## Context

BoardActions 편집 모드는 `<form>`이 아닌 `<div>` 컨테이너에 input 2개(이름·설명)와 저장/취소 버튼을 배치한다. 프로젝트의 인라인 미니 폼 지배 관례는 `onKeyDown` Enter 핸들러(MyPageClient:124 · AddToBoardButton:372 · SearchBar:166)이고, native form 저작은 전용 페이지/대형 폼(PinCreateForm · ProfileEditForm)에서 쓰인다.

## Goals / Non-Goals

- Goal: BoardActions 편집 폼의 이름·설명 입력에서 Enter로 저장 실행.
- Non-Goal: `<form>` 요소로의 재저작(채널 전환), 시각 변경, 다른 폼 접촉.

## Decisions

### Decision 1 — onKeyDown Enter 채널로 저작 (native form 전환 기각)

인라인 미니 폼 3곳의 지배 idiom을 그대로 따른다:

```tsx
onKeyDown={(e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    handleSave();
  }
}}
```

- 대안(기각): `<div>`를 `<form onSubmit>`으로 재저작 — 인라인 미니 폼 층위의 기존 채널(onKeyDown)과 갈라져 새로운 채널 갈림을 만들고, DOM 구조 변경으로 회귀 위험이 더 큼. 보수 원칙(기존 시각 동작 보존·영향 최소) 위반.

### Decision 2 — 이름·설명 두 입력 모두에 부착

ProfileEditForm(native form)에서는 어느 텍스트 입력에서든 Enter가 암시적 제출을 트리거한다. 동등한 거동을 위해 두 입력 모두에 동일 핸들러를 부착한다. 설명 입력만 제외하면 폼 내 입력 간 거동이 갈리는 미시 비정합이 새로 생긴다.

### Decision 3 — saving 게이팅은 기존 경로 재사용

`handleSave`는 저장 버튼 `onClick`과 동일 함수다. `saving` 중 Enter 연타에 대한 방어는 별도 추가하지 않는다 — 인접 idiom(MyPageClient `handleCreate`·AddToBoardButton `handleCreateAndAdd`)도 핸들러 진입 게이팅 없이 동일 구조이며, `setSaving(true)` 이후 재진입해도 서버 idempotent 업데이트라 회귀 위험이 낮다. 관례 초과 방어 로직 추가는 scope 밖.

## Risks / Trade-offs

- 위험: 낮음. 단일 컴포넌트 지역 변경, 시각·API 비접촉.
- Trade-off: native form 채널이 아닌 onKeyDown 채널 선택 — 인라인 미니 폼 층위의 지배 관례와 정렬이 우선.
