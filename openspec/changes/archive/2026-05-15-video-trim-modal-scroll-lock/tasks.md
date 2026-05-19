# tasks

- [x] 1. `apps/web/src/components/pin/VideoTrimModal.tsx:57` ESC 핸들러 useEffect 블록(L57-63) 다음에 body scroll lock useEffect 블록 1개 추가. 본문: `document.body.style.overflow = "hidden";` cleanup: `document.body.style.overflow = "";`. deps `[]` (mount/unmount 1회).
- [x] 2. 사후 grep `body.style.overflow` apps/web/src 결과 4건(AddToBoardButton:118·:120 기존, VideoTrimModal:신규 lock·cleanup) 확인.
- [x] 3. VideoTrimModal 아래쪽 라인 번호 6줄씩 밀림 확인. 활성 백로그 항목·in_progress 후보는 라인 번호 의존성 없음. done/archive 항목은 historical record로 동기화 불필요.
