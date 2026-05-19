# proposal

## why
- `apps/web/src/components/nav/NavBar.tsx:18`의 'Fugue' 워드마크 `<span>` className이 `text-xl font-bold tracking-tight text-text-primary`로 되어 있어 `font-display`(=General Sans 700) 토큰이 빠져 있다. 결과적으로 sticky 헤더에 매 페이지마다 노출되는 브랜드 텍스트가 OS sans-serif(Inter fallback)로 렌더되고, 같은 페이지의 h1/h2 디스플레이 헤딩은 General Sans 700으로 렌더되는 위계 비대칭이 발생한다.
- DESIGN.md L17 "Display/Hero: General Sans 700"이 디스플레이/브랜드 카테고리에 General Sans 700을 직접 명시. 코드 SSoT는 사이클 21(archive/2026-05-15-token-display-font-family)·24(archive/2026-05-15-profile-header-display-font)·46(archive/2026-05-15-h2-display-font-tracking)·52(archive/2026-05-15-login-h1-display-font)에서 display 카테고리 텍스트를 `font-display tracking-tight` 페어로 표기하는 표준을 정착시켰다.
- 사후 grep `tracking-tight` 사용처 중 `font-display` 미포함 건은 NavBar.tsx:18 단일 outlier(다른 모든 `tracking-tight` 사용처는 모두 `font-display`와 페어).

## what
- NavBar.tsx:18 `<span>` className에 `font-display` 1단어 추가.

## scope
- 변경 파일: `apps/web/src/components/nav/NavBar.tsx` 1개.
- 변경 범위: L18 className 끝(혹은 사이)에 단어 1개 추가.
- 비포함: NavBar L15 로고 박스(`rounded-md`), L46 그라디언트 폴백 아바타, L60 로그인 버튼 텍스트 위계 등 인접 항목.

## references
- DESIGN.md L17 "Display/Hero: General Sans 700".
- 사이클 21 archive/2026-05-15-token-display-font-family — `--font-display` 토큰 정의 + `font-display` 유틸리티 생성.
- 사이클 24·46·52 archive — display 카테고리 텍스트의 `font-display tracking-tight` 표준 패턴 정착.
