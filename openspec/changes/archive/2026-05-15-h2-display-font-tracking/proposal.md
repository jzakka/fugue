# proposal

## summary

섹션/모달 헤딩 `<h2>` 9건 중 코드 SSoT 표준 패턴(`text-lg font-bold tracking-tight font-display`)에서 `font-display + tracking-tight`가 누락된 5건에 두 클래스를 추가해 디스플레이 헤딩 위계 일관성을 확보한다.

## background

DESIGN.md L17: "Display/Hero: General Sans 700 — 기하학적이면서 개성 있음."

직전 사이클 archive 결정:
- `archive/2026-05-15-token-display-font-family` (사이클 21) — `--font-display` 토큰화 + 8개 헤딩에 `font-display` 부여.
- `archive/2026-05-15-profile-header-display-font` (사이클 24) — ProfileHeader `<h1>`에 `font-display` 보강.

위 두 결정으로 코드 SSoT가 `text-lg font-bold tracking-tight font-display` 표준 패턴을 헤딩에 적용하는 방향으로 정착했고, 현재 h2 9건 중 4건(`pins/[id]/page.tsx:249`, `BoardGrid.tsx:14`, `AddToBoardButton.tsx:210`, `MyPageClient.tsx:67`)이 이 패턴을 정확히 사용 중이다. 그러나 나머지 5건은 `font-display`·`tracking-tight` 중 하나 또는 둘 다 누락되어 OS sans-serif 폴백으로 렌더링되며 같은 페이지 내 다른 h2와 위계 일관성이 깨진다.

## change scope

`apps/web/` 안의 5개 `<h2>` 라인에 `font-display tracking-tight` 두 클래스를 추가한다.

1. `apps/web/src/app/search/SearchClient.tsx:292` — `text-lg font-semibold mb-4` → `text-lg font-semibold mb-4 font-display tracking-tight`
2. `apps/web/src/app/search/SearchClient.tsx:306` — 동일
3. `apps/web/src/app/search/SearchClient.tsx:344` — 동일
4. `apps/web/src/components/pin/VideoTrimModal.tsx:128` — `text-lg font-bold text-text-primary` → `text-lg font-bold text-text-primary font-display tracking-tight`
5. `apps/web/src/components/profile/ProfileEditForm.tsx:52` — `text-xl font-bold` → `text-xl font-bold font-display tracking-tight`

처리 범위에서 제외:
- SearchClient 3곳의 `font-semibold` → `font-bold` weight 정렬: 코드 SSoT 근거가 약해 별도 후보로 분리.
- ProfileEditForm `text-xl` → `text-lg` 사이즈 정렬: 동일 사유로 별도 후보 분리.

## impact

- 시각 변경: SearchClient h2 3곳·VideoTrimModal h2 1곳·ProfileEditForm h2 1곳의 글꼴이 시스템 sans-serif에서 General Sans로 전환되고 letter-spacing이 `tracking-tight`(-0.025em)로 좁아짐. 검색 결과 섹션 헤딩, 비디오 트림 모달 제목, 프로필 편집 헤딩에서 동일 페이지의 다른 디스플레이 헤딩과 위계가 정렬됨.
- 행동 변경: 없음.
- 회귀 위험: 매우 낮음. `font-display`·`tracking-tight`는 이미 코드베이스 다수 위치에서 사용 중인 토큰/유틸이며 의미가 안정적임.

## anti-pattern check

- L15 (Tailwind 기본 의미 덮어쓰기): 해당 안 됨. `font-display`는 `globals.css @theme inline`에서 정의한 신규 토큰이며 Tailwind 기본 `font-sans/mono` 어느 것과도 충돌하지 않음. `tracking-tight`는 Tailwind 기본 letter-spacing 유틸 그대로 사용(덮어쓰기 없음).
- L16 (radius 등급 매핑 모호): 해당 안 됨(타이포 영역).

## rollback

```
git revert <commit>
```

세 파일 5라인 단순 className 변경이라 영향 격리가 명확하다.
