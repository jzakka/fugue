## 1. 헤딩 className 보강

- [x] 1.1 `apps/web/src/components/profile/ProfileHeader.tsx:32`의 `<h1>` className `"text-2xl sm:text-3xl font-bold tracking-tight"`를 `"text-2xl sm:text-3xl font-bold tracking-tight font-display"`로 교체.

## 2. 검증

- [x] 2.1 grep으로 `font-display`가 없는 `text-2xl sm:text-3xl font-bold tracking-tight` 패턴이 `apps/web/src` 아래에 0건 남음을 확인.
- [x] 2.2 변경된 파일이 ProfileHeader.tsx 단일임을 git diff로 확인. `apps/web/` 밖 변경 없음.

## 3. 사후 기록

- [x] 3.1 `.fugue/decision-log.md`에 "ProfileHeader 닉네임 헤딩 font-display 보강" 항목 1~3줄 추가.
- [x] 3.2 `.fugue/backlog-design.yaml`에서 `design-20260515-profile-header-h1-display-font-missing` status를 `done`으로 변경 + note 추가.
- [x] 3.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-profile-header-display-font/`로 이동.
