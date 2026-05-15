## 1. 구현

- [x] 1.1 `apps/web/src/components/pin/VideoTrimModal.tsx` L121 패널 div에 `role="dialog"`, `aria-modal="true"`, `aria-labelledby="video-trim-modal-title"` 3속성 추가.
- [x] 1.2 같은 파일 L123 `<h2>`에 `id="video-trim-modal-title"` 추가.
- [x] 1.3 `apps/web/src/components/board/AddToBoardButton.tsx` L201 패널 div에 `role="dialog"`, `aria-modal="true"`, `aria-labelledby="add-to-board-modal-title"` 3속성 추가.
- [x] 1.4 같은 파일 L207 `<h2>`에 `id="add-to-board-modal-title"` 추가.

## 2. 검증

- [x] 2.1 grep `role="dialog"` 결과 2건 확인 (VideoTrimModal·AddToBoardButton 각 1건).
- [x] 2.2 grep `aria-modal="true"` 결과 2건 확인.
- [x] 2.3 grep `aria-labelledby` 결과 2건 확인.
- [x] 2.4 grep `id="video-trim-modal-title"` 1건, `id="add-to-board-modal-title"` 1건 확인.
- [x] 2.5 본 사이클 변경은 위 2 파일에 한정됨.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "모달 dialog ARIA 시맨틱 트리플 부여" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-modal-dialog-role-missing` 항목 status를 `done`으로 변경 + note 보강.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-modal-dialog-role-attributes/`로 이동.
