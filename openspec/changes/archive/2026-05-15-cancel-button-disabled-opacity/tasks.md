## 1. 구현

- [x] 1.1 `apps/web/src/app/pin/new/PinCreateForm.tsx:601` className 끝에 ` disabled:opacity-50` 토큰 추가.
- [x] 1.2 `apps/web/src/app/boards/[id]/BoardActions.tsx:99` className 끝에 동일 토큰 추가.
- [x] 1.3 `apps/web/src/components/profile/ProfileEditForm.tsx:104` className 끝에 동일 토큰 추가.

## 2. 검증

- [x] 2.1 grep `disabled:opacity-50` apps/web/src 결과 12 → 15건으로 +3 증가 확인(추정한 11→14는 베이스라인 미세 오차였음).
- [x] 2.2 변경 3 파일 외 `apps/web/` 다른 파일 변경 0건 확인 (PinCreateForm은 prior cycle 누적 변경분 외 본 사이클은 L601 단일 라인만).
- [x] 2.3 변경된 3 라인 모두 className에 disabled:opacity-50가 정확히 1회 등장 확인(git diff +/- pair 검사).

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "취소 버튼 3곳 disabled 페이드 일관" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-cancel-button-disabled-style-missing` 항목 status를 `done`으로 변경 + note 보강.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-cancel-button-disabled-opacity/`로 이동.
