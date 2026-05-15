# proposal

## summary

로그인 카드 panel의 `rounded-2xl`을 `rounded-[16px]`로 치환해 모달/패널 그룹 8건과 동일한 코드 SSoT 패턴으로 정렬한다.

## background

DESIGN.md L73-77: "Border radius: sm: 6px (inputs, alerts) / md: 10px (cards) / lg: 16px (modals, panels) / full: 9999px (buttons, chips, avatars, search bar)"

DESIGN.md L74가 "lg: 16px (modals, panels)" 두 카테고리를 동일 tier로 묶고 있고, 코드 SSoT는 8개 모달/패널 외곽 컨테이너 모두 `rounded-[16px]`를 사용한다. 로그인 페이지의 `<main>` 카드 컨테이너는 로그인 폼을 담는 panel-급 컨테이너지만 `rounded-2xl`(Tailwind 기본 1rem = 16px)로 단독 outlier 상태다. 렌더 값은 동일하지만 SSoT 표기가 어긋난다.

## change scope

`apps/web/src/app/login/page.tsx:35` 단일 라인의 className에서 `rounded-2xl`을 `rounded-[16px]`로 치환.

치환 전:
```tsx
<main className="w-full max-w-[400px] bg-surface rounded-2xl p-8 border border-border">
```

치환 후:
```tsx
<main className="w-full max-w-[400px] bg-surface rounded-[16px] p-8 border border-border">
```

처리 범위에서 제외:
- 같은 파일 L38 마스코트 로고 박스 `rounded-xl`: 결정 로그 2026-05-15 L100 `2026-05-15-login-logo-radius-non-spec`에서 rejected_self 처리됨(어느 tier인지 DESIGN.md가 명시하지 않는 마스코트 박스). 본 변경은 panel 컨테이너에만 적용.

## impact

- 시각 변경: 0건(Tailwind v4 기본 `rounded-2xl` = 1rem = 16px = `rounded-[16px]`로 렌더 결과 동일).
- 행동 변경: 없음.
- 회귀 위험: 매우 낮음. 동일한 픽셀 값을 다른 utility 표기로 치환할 뿐.
- 미래 silent regression 방지: Tailwind 기본 radius 스케일이 변경될 가능성에 대비해 픽셀 값을 명시적으로 고정.

## anti-pattern check

- L15 (Tailwind 기본 의미 덮어쓰기): 해당 안 됨. `rounded-2xl`을 사용 중인 곳을 `rounded-[16px]`로 바꾸는 것이며, Tailwind `rounded-*` 기본 클래스의 의미를 globals.css에서 덮어쓰는 행위가 아님.
- L16 (radius 등급 매핑 모호): 해당 안 됨. 로그인 `<main>` 카드는 폼 panel 컨테이너로 DESIGN.md L74 'panels' tier 직접 매핑 대상.

## rollback

```
git revert <commit>
```

단일 파일 1라인 변경이라 영향 격리가 완벽하다.
