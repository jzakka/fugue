# tasks

- [x] 1. `apps/web/src/components/nav/NavBar.tsx:18` `<span>` className 끝에 `font-display` 1단어 추가. 기존 `text-xl font-bold tracking-tight text-text-primary` → `text-xl font-bold tracking-tight text-text-primary font-display`.
- [x] 2. 사후 grep `tracking-tight` apps/web/src 결과 중 `font-display` 미포함 건 0건 확인(이전 1건 → 신규 0건).
- [x] 3. 라인 시프트 0(같은 라인 className 단어 1개 추가). 다른 후보·아카이브 항목 라인 의존성 영향 없음.
