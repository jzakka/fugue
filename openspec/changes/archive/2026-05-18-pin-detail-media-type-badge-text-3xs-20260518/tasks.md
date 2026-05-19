## 1. 미디어 타입 배지 typography 토큰 정렬

- [x] 1.1 `apps/web/src/app/pins/[id]/page.tsx:130` 미디어 타입 배지 `<span>` className에서 `text-xs` 1단어를 `text-3xs`로 교체

## 2. 검증

- [x] 2.1 `grep -rn 'text-xs' apps/web/src/app/pins/\[id\]/page.tsx`로 L130 라인에서 `text-xs` 미존재 확인
- [x] 2.2 `grep -rn 'text-3xs' apps/web/src/app/pins/\[id\]/page.tsx`로 L130 라인에서 `text-3xs` 존재 확인
- [x] 2.3 미디어 타입 배지의 다른 utility(`inline-block`, `px-3`, `py-1`, `bg-accent-subtle`, `text-accent`, `rounded-full`, `font-medium`, `font-mono`)가 모두 유지되는지 확인
