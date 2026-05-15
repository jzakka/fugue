## 1. aria-label 부여

- [x] 1.1 `apps/web/src/components/board/AddToBoardButton.tsx:212` 모달 닫기 `<button>`에 `aria-label="닫기"` 추가.
- [x] 1.2 `apps/web/src/components/nav/SearchBar.tsx:219` 최근 검색 삭제 `<button>`에 `aria-label="최근 검색에서 제거"` 추가.
- [x] 1.3 `apps/web/src/components/feed/PinCard.tsx:65` 오디오 카드 재생 `<button>`에 `aria-label="재생"` 추가.
- [x] 1.4 `apps/web/src/components/feed/PinCard.tsx:118` `ExternalLinkIcon` `<button>`에 `aria-label="원본 보기"` 추가(`title` 속성은 유지).

## 2. 검증

- [x] 2.1 grep `aria-label` 결과 4곳 모두 정확히 1줄씩 추가됨 확인(AddToBoardButton/SearchBar/PinCard x2).
- [x] 2.2 `apps/web/` 밖 변경 없음. 변경 파일 = AddToBoardButton.tsx + SearchBar.tsx + PinCard.tsx (3개).
- [x] 2.3 시각·행동 변경 없음. className/onClick/title 등 다른 속성 그대로.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "아이콘 전용 버튼 4곳 aria-label 부여" 항목 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-icon-only-buttons-no-accessible-name` 항목 status를 `done`으로 변경 + note 추가.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-icon-only-button-aria-label/`로 이동.
