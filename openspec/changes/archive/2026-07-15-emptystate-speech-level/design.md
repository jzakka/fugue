# Design: emptystate-speech-level

## Context

공유 `EmptyState` 컴포넌트(components/feed/EmptyState.tsx)를 사용하는 풀-서피스 빈 상태 4곳 중 피드 분야 필터 빈 상태만 해요체다.

- FeedContainer.tsx:165 — `<EmptyState message="이 분야의 작품이 아직 없어요">` (해요체, 유일 이탈)
- PinsGrid.tsx:146 — `"아직 등록된 작품이 없습니다"` (합쇼체)
- MyPageClient.tsx:141 — `"아직 생성된 보드가 없습니다"` (합쇼체)
- AddToBoardButton.tsx:300 — `"아직 생성된 보드가 없습니다"` (합쇼체)

해요체 어미(어요|아요|해요|예요|에요|네요|죠) grep은 apps/web/src 전수에서 FeedContainer:165 단 1건이다. 나머지 문장형 UI 카피는 전부 합쇼체(-습니다/-세요)다.

## Goals / Non-Goals

- Goal: 빈 상태 메시지 어체를 지배 관례(합쇼체)로 정렬한다.
- Non-Goal: EmptyState 컴포넌트 구조·마스코트·레이아웃·렌더 조건 변경. 다른 카피 문구 변경. 어체 컨벤션의 문서화(DESIGN.md는 apps/web/ 밖이라 루프 수정 범위가 아님 — 본 변경은 apps/web/ 내 코드 정렬만 수행).

## Decisions

### Decision 1: 문자열 치환 방향은 이탈 1건 → 지배 관례

"없어요" → "없습니다"로 정렬한다. 역방향(전체를 해요체로) 정렬은 변경 폭 33+건·기존 머지 결정들(합쇼체 카피 다수가 QA 통과·머지됨)과 충돌하므로 기각.

변경:

```tsx
// apps/web/src/components/feed/FeedContainer.tsx:165
- <EmptyState message="이 분야의 작품이 아직 없어요">
+ <EmptyState message="이 분야의 작품이 아직 없습니다">
```

### Decision 2: 문형 프레임은 기존 문구 유지

role-identical 3곳의 프레임 "아직 ~ 없습니다"에 맞춰 "이 분야의 작품이 아직 없습니다"로 한다. 어순 재배열("아직 이 분야의 작품이 없습니다")은 불필요한 추가 변경이므로 기각 — 어체만 정렬한다.

## Risks / Trade-offs

- 테스트가 기존 문구를 단언하면 실패 → 해당 단언 문자열 동기 갱신 (사전 grep으로 확인).
- 그 외 회귀 없음: 렌더 조건·props·스타일 불변.
