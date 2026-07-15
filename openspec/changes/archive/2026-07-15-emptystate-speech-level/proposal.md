# Proposal: emptystate-speech-level

## Why

빈 상태 메시지의 어체가 표면 간 갈린다. 공유 `EmptyState` 컴포넌트를 쓰는 role-identical 빈 상태 4곳 중 피드 분야 필터 빈 상태(FeedContainer.tsx:165)만 해요체("이 분야의 작품이 아직 없어요")이고, 나머지 3곳(PinsGrid:146 "아직 등록된 작품이 없습니다"·MyPageClient:141 "아직 생성된 보드가 없습니다"·AddToBoardButton:300 동일)과 그 외 모든 문장형 UI 카피는 합쇼체(-습니다/-세요)다. 지배 관례(합쇼체 전수)로 정렬하여 어체 일관성을 확보한다.

## What Changes

- `apps/web/src/components/feed/FeedContainer.tsx:165`의 EmptyState 메시지를 "이 분야의 작품이 아직 없어요" → "이 분야의 작품이 아직 없습니다"로 변경 (문자열 1건)
- 관련 테스트가 이 문구를 단언하면 함께 갱신

## Capabilities

### Modified Capabilities

- `feed`: 피드 분야 필터 빈 상태 메시지 카피의 어체를 합쇼체로 정렬 — 기존 빈 상태 동작(EmptyState 렌더 조건·마스코트·레이아웃)은 불변, 메시지 문구만 변경

### New Capabilities

없음.

## Impact

- **사용자 영향**: 작품이 없는 분야 필터 선택 시 노출되는 빈 상태 카피 한 줄의 어체만 변경. 시각 구조·동작 무변경.
- **회귀 위험**: 낮음(risk 1). 문자열 1건 변경, 렌더 조건·컴포넌트 API 불변.
- **롤백**: 문자열 원복 1커밋.
