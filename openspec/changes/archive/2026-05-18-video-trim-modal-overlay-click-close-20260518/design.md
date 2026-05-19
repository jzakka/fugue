## Context

VideoTrimModal과 AddToBoardButton 내부 `BoardSelectModal`은 둘 다 fixed overlay + 내부 dialog div 구조의 modal이다. 두 모달 모두 사이클 25에서 `role="dialog"` + `aria-modal` + `aria-labelledby` 트리플을, 사이클 56에서 body scroll lock과 Escape 닫기를 받았다. 그러나 마우스 사용자의 닫기 경로(overlay click)는 AddToBoardButton에만 구현되어 있다(`L141-145`, `L195`). VideoTrimModal의 overlay div(L135)는 onClick 핸들러가 없어 modal 바깥 dim 영역을 클릭해도 닫히지 않는다.

VideoTrimModal은 AddToBoardButton과 달리 사용자가 modal 내부 track에서 핸들을 드래그하는 인터랙션이 있다(`drag` state, L36). Escape 닫기도 L57-63에서 `if (e.key === "Escape" && !drag) onCancel()`로 드래그 중에는 닫지 않도록 가드한다. overlay click 닫기도 같은 가드 적용이 필요하다(window 드래그가 모달 바깥으로 빠져나가는 경우 우발 닫힘 회피).

## Goals / Non-Goals

**Goals:**
- VideoTrimModal에 마우스 사용자의 modal 닫기 경로를 추가.
- AddToBoardButton의 `panelRef + handleOverlayClick + onClick` 패턴을 1:1 재사용해 두 modal의 닫기 경로 일관성 확보.
- 드래그 중 우발 닫기 차단(Escape 가드 L59와 동일 정책).

**Non-Goals:**
- AddToBoardButton 측 변경.
- Initial focus 이동, Focus trap 구현(사이클 56 decision-log에서 별도 후보로 분리 유지 명시).
- 다른 modal 컴포넌트(현재는 두 모달뿐) 변경.
- VideoTrimModal의 기존 인터랙션(track 드래그, 핸들 조작, video preview) 변경.

## Decisions

### Decision 1 — AddToBoardButton 패턴 1:1 복제
AddToBoardButton(`L99 panelRef`, `L141-145 handleOverlayClick`, `L195 onClick={handleOverlayClick}`) 패턴을 VideoTrimModal에 그대로 옮긴다.

- `const panelRef = useRef<HTMLDivElement>(null);`를 기존 `videoRef`/`trackRef` 옆(L29-30 근처)에 선언.
- 내부 dialog div(L136-141)에 `ref={panelRef}` 부착.
- overlay div(L135)에 `onClick={handleOverlayClick}` 부착.

**대안:**
- onClick을 overlay div가 아니라 별도 backdrop layer에 두는 방식 — AddToBoardButton은 L194에서 단일 fixed overlay div에 onClick을 두고 내부 absolute backdrop은 시각만 담당. 같은 구조 채택.
- `onClick={(e) => e.target === e.currentTarget && onCancel()}` 한 줄 inline 비교 — `panelRef.current.contains(e.target as Node)` 체크가 dialog 내부 어디를 클릭해도 닫히지 않음을 보장하므로 더 안전. AddToBoardButton 정착 패턴과 일관.

### Decision 2 — 드래그 가드 `if (!drag)` 추가
AddToBoardButton의 `handleOverlayClick`은 단순히 `panelRef.current.contains` 체크만 한다. VideoTrimModal은 핸들/window 드래그(`drag: "start" | "end" | "window" | null`) 중에 PointerLeave가 발생하면 onPointerLeave 핸들러로 드래그 종료가 호출되긴 하지만, 사용자가 window 드래그 중 마우스가 dialog 영역 바깥으로 빠지면서 동시에 pointerup이 발생하는 케이스가 있다. Escape 처리 L59와 동일한 `!drag` 가드를 적용해 드래그 컨텍스트 중에는 overlay click도 무시한다.

함수 본문:
```ts
function handleOverlayClick(e: React.MouseEvent) {
  if (drag) return;
  if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
    onCancel();
  }
}
```

**대안:**
- 드래그 가드 없이 AddToBoardButton 패턴 그대로 복제 — VideoTrimModal은 핸들 조작 중 우발 클릭 위험이 AddToBoardButton보다 크다(트리밍 작업 손실). Escape 가드 일관성도 깨진다.

## Risks / Trade-offs

- **[리스크] 드래그 종료 후 즉시 클릭이 dialog 바깥에서 일어나는 케이스가 누락될 수 있음** → `onPointerUp`이 `setDrag(null)`을 호출한 직후 같은 이벤트 루프에서 click 이벤트가 dialog 외부에 도달하면 `!drag` 가드 통과 후 닫힘. 이는 의도된 동작(드래그 종료 후 명시적 dim 클릭은 닫기 의도). 우발 닫기 우려는 사용자가 드래그 중에 dim 영역으로 빠져나가는 케이스에 한정.
- **[리스크] panelRef가 ref 할당 직후 첫 render에 null일 수 있음** → handleOverlayClick은 onClick 이벤트 핸들러로 user gesture 이후에만 실행되므로 ref 할당이 이미 완료된 상태. `panelRef.current && ...` 가드로 null-safe.
- **[Trade-off] AddToBoardButton 패턴과 미세한 분기**: `!drag` 가드는 VideoTrimModal 고유 상태에 종속이라 100% 동일 코드는 아님. 두 modal 공통 함수로 추출하지 않음(현재 modal 컴포넌트 2개, 추출 effort > 단순 복제).
- **[리스크] 환경 제약으로 dev 서버 시각 검증 미수행** → Ralph 루프 환경에 `node_modules` 미설치. 코드 검증(grep, diff)으로 대체. 패턴이 AddToBoardButton 1:1 복제라 시각 회귀 위험 낮음.

## Migration Plan

1. `apps/web/src/components/pin/VideoTrimModal.tsx` 4단계 편집.
2. git commit 1건.
3. 롤백: `git revert <commit-sha>` 1회.
