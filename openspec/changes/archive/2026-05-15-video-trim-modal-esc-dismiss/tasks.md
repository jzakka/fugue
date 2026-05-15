# tasks

- [x] 1. `apps/web/src/components/pin/VideoTrimModal.tsx:57-62` ESC 핸들러 useEffect 블록 추가(기존 L51-55 useEffect 다음). handleKeyDown 본문: `if (e.key === 'Escape' && !drag) onCancel()`. deps: `[drag, onCancel]`. cleanup으로 `window.removeEventListener` 등록.
- [x] 2. 사후 grep `key === "Escape"` apps/web/src 결과 3건(VideoTrimModal:59 신규·AddToBoardButton:127·SearchBar:169) 확인.
- [x] 3. VideoTrimModal 아래쪽 라인 번호 7줄씩 밀림 확인. 활성 백로그 항목·in_progress 후보는 라인 번호 의존성 없음. done/archive 항목은 historical record로 동기화 불필요.
