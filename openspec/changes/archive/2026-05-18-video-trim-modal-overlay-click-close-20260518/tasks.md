## 1. 구현

- [x] 1.1 `apps/web/src/components/pin/VideoTrimModal.tsx`의 `videoRef`/`trackRef` 옆(L29-30 근처)에 `const panelRef = useRef<HTMLDivElement>(null);` 추가
- [x] 1.2 내부 dialog div(L136-141)에 `ref={panelRef}` 부착 (기존 role/aria/className 유지)
- [x] 1.3 컴포넌트 본문에 `handleOverlayClick(e: React.MouseEvent)` 함수 추가 — `if (drag) return; if (panelRef.current && !panelRef.current.contains(e.target as Node)) onCancel();`
- [x] 1.4 overlay div(L135)에 `onClick={handleOverlayClick}` 부착 (기존 className 유지)

## 2. 검증

- [x] 2.1 grep `panelRef` `apps/web/src/components/pin/VideoTrimModal.tsx` → 3건(선언 L31 + 사용 L133 + ref 부착 L148)
- [x] 2.2 grep `handleOverlayClick` `apps/web/src/components/pin/VideoTrimModal.tsx` → 2건(선언 L131 + onClick 부착 L145)
- [x] 2.3 `git diff apps/web/src/components/pin/VideoTrimModal.tsx` 검토 — +13 -1 hunk, ref 선언 추가 / handleOverlayClick 함수 추가 / overlay div onClick 추가 / dialog div ref 추가만 변경
- [x] 2.4 기존 inner dialog div의 `role="dialog"`, `aria-modal="true"`, `aria-labelledby="video-trim-modal-title"`, className 유지 확인
- [x] 2.5 기존 Escape 처리(L57-63), body scroll lock(L65-70), drag state(L36), pxToTime/onMove/onUp 핸들러 미수정 확인
- [x] 2.6 AddToBoardButton의 `handleOverlayClick`(L141-145) · `panelRef`(L99) · overlay `onClick`(L195) 미수정 확인
