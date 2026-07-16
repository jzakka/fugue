# Design: img-onerror-degrade

## Context

외부 URL `<img>` 12표면 census: onError degrade 보유 5곳(PinCard:88 핀 미디어·PinCard:167 avatar·PinCard:178 favicon·SearchBar:284 og_image 썸네일·ProfileEditForm:105 avatar 미리보기 — 전부 `e.currentTarget.style.display = "none"` 또는 `(e.target as HTMLImageElement).style.display = "none"` 인라인 핸들러), 부재 7곳. 부재 7곳 중 3곳은 `"use client"` 컴포넌트(SearchClient·BoardCover·AddToBoardButton), 4개 img는 서버 컴포넌트(NavBar·ProfileHeader·pins/[id] ×2) 내부라 onError(클라이언트 이벤트 핸들러) 직접 부착 불가.

## Goals / Non-Goals

- Goals
  - 로드 실패 degrade idiom(display:none hide)을 외부 URL img 전 표면에 균일 적용.
  - 정상 로드 시 렌더 결과(클래스·속성·DOM 구조·레이아웃) 완전 불변.
- Non-Goals
  - 실패 시 gradient placeholder 전환(하이드 대신 대체 시각) — 기존 보유 5곳의 지배 관례가 hide이므로 관례 정렬이 최소·보수적. placeholder 전환은 새 시각 동작 도입이라 범위 밖.
  - 기존 보유 5곳의 핸들러 통합/리팩터링 — 비접촉.
  - srcset/loading/fetchpriority 등 다른 이미지 속성 — 별개 축(L83/L91 baseline).

## Decisions

### Decision 1 — 클라이언트 컴포넌트 3곳은 인라인 onError 핸들러 추가

기존 5곳의 지배 idiom이 인라인 핸들러이므로 동일하게 정렬한다.

```tsx
onError={(e) => {
  e.currentTarget.style.display = "none";
}}
```

- `SearchClient.tsx:328` creator avatar, `BoardCover.tsx:34` 콜라주 슬롯 img, `AddToBoardButton.tsx:313` 미니커버 img에 위 핸들러만 추가. 다른 속성 무변경.
- BoardCover 콜라주는 img가 `bg-surface-elevated` div 래퍼 안에 있어 hide 시 빈 슬롯과 동일한 표면이 노출된다(L239 기존 빈 슬롯 degrade와 동종).
- 대안(공용 컴포넌트로 전면 교체)은 기각: 기존 5곳이 인라인 idiom이므로 클라이언트 사이트는 인라인이 관례 정렬이고, 전면 교체는 리팩터링 범위 확대.

### Decision 2 — 서버 컴포넌트 내 4개 img는 `HideOnErrorImage` 클라이언트 컴포넌트로 대체

`apps/web/src/components/ui/HideOnErrorImage.tsx` 신설:

```tsx
"use client";

export default function HideOnErrorImage({
  alt = "",
  ...props
}: React.ImgHTMLAttributes<HTMLImageElement>) {
  return (
    <img
      {...props}
      alt={alt}
      onError={(e) => {
        e.currentTarget.style.display = "none";
      }}
    />
  );
}
```

- 적용 4곳: `NavBar.tsx:61`(user avatar), `ProfileHeader.tsx:18`(creator avatar), `pins/[id]/page.tsx:43`(핀 미디어 image case), `pins/[id]/page.tsx:202`(creator avatar). `<img` 태그명만 교체하고 src/alt/className/기타 속성은 그대로 전달.
- `alt` 기본값 `""` + 명시 전달로 jsx-a11y alt-text 스프레드 경고 회피(4곳 모두 기존 alt 값을 그대로 전달: avatar 3곳 `""`, 핀 미디어 `pin.title`).
- 대안(서버 컴포넌트 전체를 클라이언트로 전환)은 기각: NavBar·ProfileHeader·pins 상세는 서버 데이터 페치 컴포넌트라 전환 비용·회귀 위험이 큼. leaf img만 클라이언트 경계로 내리는 것이 Next.js App Router 표준 패턴.

### Decision 3 — 실패 시 동작은 hide 단일 (placeholder 전환 없음)

- 보수 원칙(기존 시각 동작 보존·영향 최소): 기존 5곳이 전부 hide-only이므로 신규 7곳도 hide-only. avatar hide 시 인접 텍스트(nickname 등)는 유지되어 식별 가능.
- backlog note의 "gradient 폴백 전환도 검토"는 기각 — 새 시각 동작 도입은 지배 관례 정렬 범위를 벗어나고, 기존 5곳과 신규 7곳의 실패 시 거동이 다시 갈리게 된다.

## Risks / Trade-offs

- ProfileHeader avatar(w-20/24) hide 시 헤더 좌측이 비어 레이아웃이 이동한다 → 실패 케이스 한정이고 기존 PinCard avatar hide 와 동일한 trade-off(지배 관례). 정상 케이스 무영향.
- `HideOnErrorImage`는 클라이언트 번들에 소형 컴포넌트 1개 추가 → 무시 가능한 크기.
- 하이드레이션 전 로드 실패 이미지: Next.js가 hydration 시 이벤트를 부착하기 전 발생한 error 이벤트는 놓칠 수 있음 → 기존 보유 5곳(전부 클라이언트)과 동일한 한계로 신규 회귀 아님.
