## 1. VideoTrimModal 구현

- [x] 1.1 `apps/web/src/components/pin/VideoTrimModal.tsx` 내부 dialog div(L152-158, role=dialog 트리플 + ref={panelRef})에 `tabIndex={-1}` 속성 추가 (L156)
- [x] 1.2 컴포넌트 본문에 `useEffect(() => { panelRef.current?.focus(); }, []);` 한 블록 추가 (L73-75, 기존 body scroll lock useEffect L66-71 직후)

## 2. AddToBoardButton 구현

- [x] 2.1 `apps/web/src/components/board/AddToBoardButton.tsx` panel div(L206-213, role=dialog 트리플 + ref={panelRef})에 `tabIndex={-1}` 속성 추가 (L211)
- [x] 2.2 `BoardSelectModal` 본문에 `useEffect(() => { panelRef.current?.focus(); }, []);` 한 블록 추가 (L124-127, 기존 body scroll lock useEffect L117-122 직후)

## 3. 검증

- [x] 3.1 grep `tabIndex` `apps/web/src/components/pin/VideoTrimModal.tsx` → 1건 매칭(L156)
- [x] 3.2 grep `tabIndex` `apps/web/src/components/board/AddToBoardButton.tsx` → 1건 매칭(L211)
- [x] 3.3 grep `panelRef.current?.focus` 두 파일 → 각 1건 매칭(VideoTrimModal L74, AddToBoardButton L126)
- [x] 3.4 `git diff` 검토 — 두 파일에 panel div `tabIndex={-1}` 1단어 + 신규 useEffect 1블록만 본 사이클 변경(VideoTrimModal의 다른 hunk는 cycle 70 잔여, AddToBoardButton +6 -0)
- [x] 3.5 두 modal panel div의 기존 `ref={panelRef}` · `role="dialog"` · `aria-modal="true"` · `aria-labelledby` · className 유지 확인
- [x] 3.6 AddToBoardButton L316 conditional input `autoFocus` 속성 미수정 확인 (grep autoFocus → L316 1건 유지)
- [x] 3.7 VideoTrimModal Escape 처리(L57-63) · body scroll lock(L66-71) · overlay click 닫기(cycle 70 L132-137) · drag state 미수정 확인
