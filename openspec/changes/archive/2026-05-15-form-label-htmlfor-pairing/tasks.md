## 1. 구현

- [x] 1.1 `apps/web/src/app/pin/new/PinCreateForm.tsx` 제목 페어: L435 label에 `htmlFor="pin-title"`, L438 input에 `id="pin-title"` 추가.
- [x] 1.2 같은 파일 설명 페어: L451 label에 `htmlFor="pin-description"`, L453 textarea에 `id="pin-description"` 추가.
- [x] 1.3 같은 파일 원본 URL 페어: L464 label에 `htmlFor="pin-url"`, L468 input에 `id="pin-url"` 추가.
- [x] 1.4 `apps/web/src/components/profile/ProfileEditForm.tsx` 닉네임 페어: L62 label에 `htmlFor="profile-nickname"`, L64 input에 `id="profile-nickname"` 추가.
- [x] 1.5 같은 파일 아바타 URL 페어: L75 label에 `htmlFor="profile-avatar-url"`, L79 input에 `id="profile-avatar-url"` 추가.

## 2. 검증

- [x] 2.1 grep `htmlFor` 결과 5건 확인 (PinCreateForm 3건 + ProfileEditForm 2건).
- [x] 2.2 grep `id="(pin|profile)-` 결과 5건 확인 (id="pin-title", id="pin-description", id="pin-url", id="profile-nickname", id="profile-avatar-url").
- [x] 2.3 각 페어의 htmlFor 값과 id 값이 일치함을 동일 grep 출력에서 교차 확인.
- [x] 2.4 본 사이클 변경은 위 2 파일에 한정됨(다른 변경은 이전 사이클 누적분).

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "폼 라벨-컨트롤 htmlFor 연결" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-form-label-htmlfor-missing` 항목 status를 `done`으로 변경 + note 보강.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-form-label-htmlfor-pairing/`로 이동.
