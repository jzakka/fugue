## 1. 토큰 정의

- [x] 1.1 `apps/web/src/app/globals.css`의 `@theme inline` 블록 내 `--font-mono: 'Geist Mono', monospace;` 다음에 `--text-2xs: 0.6875rem;`와 `--text-3xs: 0.625rem;` 두 줄 추가.

## 2. 매직값 → 토큰 치환

- [x] 2.1 `apps/web/src/components/feed/PinCard.tsx:181` `<span>` className에서 `text-[10px]` → `text-3xs`.
- [x] 2.2 `apps/web/src/components/nav/SearchBar.tsx:292` `<span>` className에서 `text-[10px]` → `text-3xs`.

## 3. 검증

- [x] 3.1 grep으로 `text-\[10px\]` 패턴이 `apps/web/src` 아래에 0건 남음을 확인.
- [x] 3.2 grep으로 `text-2xs` 사용처(VideoTrimModal 2건)·`text-3xs` 사용처(PinCard·SearchBar 각 1건) 총 4건 확인.
- [x] 3.3 `apps/web/` 밖 변경 없음을 git diff로 확인. 변경 파일 = globals.css + PinCard.tsx + SearchBar.tsx (3개).

## 4. 사후 기록

- [x] 4.1 `.fugue/decision-log.md`에 "폰트 스케일 2xs/3xs 토큰화" 항목 1~3줄 추가.
- [x] 4.2 `.fugue/backlog-design.yaml`에서 `design-20260515-text-scale-tokens-missing` 항목 status를 `done`으로 변경 + note 추가.
- [x] 4.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-text-scale-tokens-2xs-3xs/`로 이동.
