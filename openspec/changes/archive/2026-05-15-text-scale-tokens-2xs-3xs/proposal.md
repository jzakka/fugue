# Font-size 스케일 토큰 --text-2xs / --text-3xs 추가

## 배경

DESIGN.md L34-35는 다음 두 단계를 명시한다:

> 2xs: 11px / 0.6875rem (timestamps, duration)
> 3xs: 10px / 0.625rem (tags, category labels)

Tailwind 기본 스케일에는 `text-2xs` / `text-3xs`가 없다. `apps/web/src/app/globals.css`의 `@theme inline` 블록도 폰트 사이즈 토큰을 정의하지 않는다. 결과:

- `VideoTrimModal.tsx:144, 202` — `text-2xs` 클래스 사용 중이지만 토큰이 없어 효과 없음(부모 폰트 사이즈로 폴백).
- `PinCard.tsx:181` / `SearchBar.tsx:292` — `text-[10px]` 매직값 직접 사용. DESIGN.md L35 3xs 정의 무시.

## 변경 범위

1. **globals.css** — `@theme inline` 블록에 두 토큰 추가(Tailwind v4가 자동으로 `text-2xs` / `text-3xs` 유틸리티 생성):
   ```css
   --text-2xs: 0.6875rem;
   --text-3xs: 0.625rem;
   ```
2. **`apps/web/src/components/feed/PinCard.tsx:181`** — `text-[10px]` → `text-3xs`
3. **`apps/web/src/components/nav/SearchBar.tsx:292`** — `text-[10px]` → `text-3xs`

## 사용자 영향

- `VideoTrimModal` 시간 라벨: 부모 폰트 사이즈 → 11px로 명시적 축소. 트림 모달의 작은 시간 인디케이터가 의도된 작은 크기로 렌더링됨.
- `PinCard` 태그 칩 / `SearchBar` 드롭다운 미디어 타입 배지: 폰트 사이즈 값 동일(10px). 의미만 매직값 → 토큰으로 회복.

## anti-pattern L15 검토

L15 = "토큰 추가 + 기본 클래스 의미 덮어쓰기 단일 항목으로 묶지 마라". `text-2xs` / `text-3xs`는 Tailwind 기본 스케일에 없는 신규 단계. 기본 의미 덮어쓰기에 해당하지 않음. L15 적용 대상 아님.

decision-log 2026-05-15 `타이포 스케일 일괄 등록 변경 반려` 항목이 명시한 "두 항목 분리" 가이드와 일치: 본 변경은 신규 토큰 추가만 다루고, `text-sm/base/3xl` 기본값 덮어쓰기는 별도 후보로 유지.

## 롤백 절차

1. globals.css에서 추가한 두 토큰 줄 제거.
2. PinCard / SearchBar 각 1줄에서 `text-3xs` → `text-[10px]` 환원.
