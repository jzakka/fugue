## 1. 토큰 정렬

- [x] 1.1 `apps/web/src/app/pins/[id]/page.tsx:152` 태그 칩 `<span>` className의 `text-xs`를 `text-3xs`로 교체. 그 외 utility(`px-2.5 py-1 bg-accent-subtle text-text-muted rounded-full font-mono`)는 변경하지 않는다.

## 2. 검증

- [x] 2.1 변경 후 `apps/web/src/app/pins/[id]/page.tsx` diff가 1라인 1단어 교체에 한정되는지 확인.
- [x] 2.2 `apps/web/src/app/globals.css` `@theme inline`에 `--text-3xs: 0.625rem`이 존재함을 확인(이미 archive/2026-05-15-text-scale-tokens-2xs-3xs에서 정의됨).
- [x] 2.3 핀 상세 페이지(`/pins/<id>`)의 태그 칩이 creator name(`text-xs`)·timestamp(`text-2xs`)보다 작게 렌더링되는 위계가 유지되는지 시각 확인.
