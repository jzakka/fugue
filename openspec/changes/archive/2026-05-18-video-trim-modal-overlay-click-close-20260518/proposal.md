## Why

`apps/web/src/components/pin/VideoTrimModal.tsx`는 modal dialog인데 overlay 바깥 영역(`bg-black/70 backdrop-blur-sm` dim)을 마우스로 클릭해도 닫히지 않는다. 동일한 modal 컴포넌트인 `apps/web/src/components/board/AddToBoardButton.tsx`는 `handleOverlayClick` + `panelRef.current.contains` 체크 패턴으로 overlay click 닫기를 표준화한 상태(L141-145, L195). VideoTrimModal만 마우스 닫기 경로가 누락된 outlier. 사이클 56 archive(`2026-05-15-video-trim-modal-scroll-lock`)의 decision-log note(`.fugue/decision-log.md:23`)가 "overlay click 닫기·Initial focus·Focus trap은 별도 후보로 분리 유지"라고 본 후보를 명시적으로 예약했다.

## What Changes

- VideoTrimModal에 `panelRef = useRef<HTMLDivElement>(null)` 추가.
- 내부 dialog div(L136-141)에 `ref={panelRef}` 부착.
- `handleOverlayClick(e: React.MouseEvent)` 함수 추가 — `if (!drag && panelRef.current && !panelRef.current.contains(e.target as Node)) onCancel()`. AddToBoardButton 패턴(`L141-145`)을 기반으로 하되, VideoTrimModal 고유의 핸들/window 드래그 상태(`drag !== null`) 중에는 닫기 차단(Escape 처리 L57-63과 동일한 가드).
- overlay div(L135)에 `onClick={handleOverlayClick}` 추가.

## Capabilities

### New Capabilities
- `design-tokens`: 디자인 시스템 SSoT(DESIGN.md + 코드 SSoT) 일관성 capability. archive 누적 패턴(2026-05-18 design-tokens archive 10건)에 따라 디자인 트랙 변경 사항은 이 capability에 누적한다. 이번 사이클은 modal 닫기 인터랙션 일관성 요구사항을 추가한다.

### Modified Capabilities
(없음)

## Impact

- 변경 파일: `apps/web/src/components/pin/VideoTrimModal.tsx` 1개.
- 변경 범위: `useRef` 1개 + `ref={panelRef}` 1개 + `handleOverlayClick` 함수 1개 + `onClick` 1개. 4단계 변경.
- 사용자 영향: 동영상 트리밍 modal 열린 상태에서 dim 영역 마우스 클릭 시 modal 닫힘 → AddToBoardButton과 닫기 패턴 일관성 회복. 드래그 중 우발 클릭은 `!drag` 가드로 차단되어 트리밍 작업 중 클릭으로 닫히지 않음.
- 비포함: AddToBoardButton 측 변경, Initial focus, Focus trap, 다른 modal 컴포넌트.
- 롤백: git revert로 단일 커밋 되돌리기.
