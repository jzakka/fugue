# proposal

## 변경 대상
- `apps/web/src/components/pin/VideoTrimModal.tsx` — ESC 키로 모달을 닫는 `useEffect` 한 블록 추가.

## 변경 내용
기존 `useEffect` 블록들 사이(L41-55 영역 인접)에 다음 패턴 추가:

```tsx
useEffect(() => {
  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape" && !drag) onCancel();
  }
  window.addEventListener("keydown", handleKeyDown);
  return () => window.removeEventListener("keydown", handleKeyDown);
}, [drag, onCancel]);
```

- `drag !== null`(슬라이더 핸들 드래그 진행 중)일 때 ESC를 무시. 드래그 미완료 상태에서 외부 이벤트로 작업 데이터를 손실시키지 않기 위한 가드.
- 의존성: `drag`(가드 평가), `onCancel`(stale closure 방지).

## 명세 인용
- WAI-ARIA Authoring Practices Guide — Dialog (Modal) Pattern: "Escape: Closes the dialog."
- WCAG 2.1.1 Keyboard (Level A): 모든 기능은 키보드 인터페이스를 통해 사용 가능해야 함. 모달 닫기 동작도 마우스/터치 외에 키보드 경로 필수.

## 코드 SSoT 인용
- `apps/web/src/components/board/AddToBoardButton.tsx:124-131` 동일 패턴 직접 구현. 본 변경은 그 블록의 1:1 복제 + drag 가드 추가.
- `apps/web/src/components/nav/SearchBar.tsx:169` 드롭다운 ESC 닫기(다른 컨텍스트지만 같은 키 매핑).

## 사이클 25 archive와의 관계
사이클 25 `archive/2026-05-15-modal-dialog-role-attributes`는 두 모달에 `role=dialog` + `aria-modal=true` + `aria-labelledby` 트리플을 부여하고, decision-log L48에 "ESC 처리/focus trap/initial focus 같은 keyboard interaction 항목은 별도 후보로 분리(scope 한정)"라고 미래 처리를 예약. 본 변경은 그 잔여 갭 중 ESC 처리 1항목을 처리.

## 사용자 영향
- 영상 핀 생성 흐름에서 VideoTrimModal이 열려 있을 때 ESC 키 1회로 모달 종료(취소). 기존에는 Tab 으로 '취소' 버튼까지 도달한 뒤 Enter/Space 필요했음.
- 드래그 진행 중(시작/종료/윈도 핸들 잡은 상태)에는 ESC가 무시되어 사용자가 의도치 않게 트림 작업을 잃지 않음.
- 동작 변경: 모달이 열린 동안 페이지 전역(`window`)에서 ESC 키가 onCancel을 트리거. 같은 페이지의 다른 ESC 핸들러는 없음(검색 페이지 외 SearchBar 드롭다운만 존재하고 VideoTrimModal은 핀 생성 페이지에서만 사용).
- 시각 변경 없음.

## 변경 범위
- 1개 파일: `apps/web/src/components/pin/VideoTrimModal.tsx`
- 약 7~9줄 추가 (useEffect 블록 + 가드 + cleanup + deps)
- `apps/api/` 미접근. 다른 컴포넌트 미접근.

## 롤백 절차
`git revert` 또는 추가된 `useEffect` 블록 제거.

## anti-pattern 자기 검사
- L15(Tailwind 기본 의미 덮어쓰기 vs 신규 토큰 추가): 적용 안됨. 토큰/유틸리티 영역 아닌 동작 추가.
- L16(DESIGN.md radius scale 외 자의적 등급 매핑): 적용 안됨. radius scale 영역 아님.

## 자체 리뷰 #2 사전 차단
- "drag 중 ESC 무시" 가드가 명세에 없는 자의적 결정인지 검토: `onPointerUp` 핸들러(L113)가 이미 drag 미완료 상태에서 외부 이벤트로 종료될 때의 처리 정신을 갖고 있음. 또한 ESC = onCancel 매핑은 모달 dismiss 표준(WAI-ARIA APG)이지 슬라이더 드래그 취소 표준이 아니므로 드래그 중에는 별개 인터랙션 컨텍스트로 보는 것이 자연스러움. drag 가드를 빼면 사용자가 핸들 놓는 도중 우연히 ESC가 입력되면 작업 데이터를 잃는 회귀가 발생. 가드 포함이 디자인 결정이 아닌 회귀 방지 책임.
