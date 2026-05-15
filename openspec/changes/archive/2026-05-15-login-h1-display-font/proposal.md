# proposal

## summary

로그인 페이지 h1 "작품으로 만나다"의 className에 `font-display` 1단어를 추가해 visible h1 6건 모두 동일한 General Sans 디스플레이 폰트로 렌더링되도록 정렬한다.

## background

DESIGN.md L17: "Display/Hero: General Sans 700 — 기하학적이면서 개성 있음. 한글 대체: Pretendard Bold"

직전 archive 결정들이 코드 SSoT를 정착시킨 흐름:
- `archive/2026-05-15-token-display-font-family` (사이클 21) — `--font-display` 토큰화 + 8개 헤딩(inline `fontFamily` 사용처)에 `font-display` 부여.
- `archive/2026-05-15-profile-header-display-font` (사이클 24) — ProfileHeader h1이 사이클 21 grep 그물을 빠져나간 잔여 갭(원래 inline fontFamily 없었음)을 보강.
- `archive/2026-05-15-h2-display-font-tracking` (사이클 48) — h2 5건에 `font-display + tracking-tight` 보강.

본 사이클에서 grep `<h1` 7건 측정 결과 visible 6건(sr-only 1건 제외) 중 5건이 `font-display`를 갖고 1건(로그인 페이지 h1)만 누락이 확인됨. 사이클 24의 ProfileHeader 보강과 정확히 동일한 잔여 갭 패턴.

## change scope

`apps/web/src/app/login/page.tsx:41` 단일 라인 className에 `font-display` 1단어 추가.

치환 전:
```tsx
<h1 className="text-2xl font-bold text-text-primary tracking-tight">
  작품으로 만나다
</h1>
```

치환 후:
```tsx
<h1 className="text-2xl font-bold text-text-primary tracking-tight font-display">
  작품으로 만나다
</h1>
```

## impact

- 시각 변경: 로그인 페이지 진입 시 첫 화면 헤딩 "작품으로 만나다"의 글꼴이 시스템 sans-serif에서 General Sans로 전환. 사이트 다른 디스플레이 헤딩(pin 상세/검색/보드/프로필 등)과 동일한 위계로 정렬.
- 행동 변경: 없음.
- 회귀 위험: 매우 낮음. 코드베이스 다수 위치에서 사용 중인 `font-display` 토큰 1단어 추가.

## anti-pattern check

- L15 (Tailwind 기본 의미 덮어쓰기): 해당 안 됨. `font-display`는 `globals.css @theme inline`에서 정의한 신규 토큰.
- L16 (radius 등급 매핑 모호): 해당 안 됨(타이포 영역).

## rollback

```
git revert <commit>
```

단일 파일 1라인 변경이라 영향 격리가 완벽하다.
