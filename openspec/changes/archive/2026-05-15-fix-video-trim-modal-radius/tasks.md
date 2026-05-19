## 1. radius 교체

- [x] 1.1 `apps/web/src/components/pin/VideoTrimModal.tsx:121`의 외곽 div className에서 `rounded-[12px]` → `rounded-[16px]`.

## 2. 검증

- [x] 2.1 grep으로 변경 후 `rounded-[12px]`이 VideoTrimModal에 남지 않음 확인.
- [x] 2.2 AddToBoardButton.tsx의 모달 패널이 여전히 `rounded-[16px]`임을 확인(비교 기준).

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "VideoTrimModal 모달 radius 16px 정렬" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-video-trim-modal-radius` 항목 status를 `done`으로 변경 + note 추가.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-fix-video-trim-modal-radius/`로 이동.
