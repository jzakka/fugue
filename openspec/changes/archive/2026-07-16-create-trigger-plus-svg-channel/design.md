# Design: create-trigger-plus-svg-channel

## Context

생성 트리거 plus 마크 census 3곳:

| site | 채널 | 접근 이름 |
|------|------|----------|
| NavBar.tsx:37 `+ 핀 생성` | 텍스트 글리프 접두 | "+ 핀 생성" |
| MyPageClient.tsx:80 `+ 새 보드` | 텍스트 글리프 접두 | "+ 새 보드" |
| AddToBoardButton.tsx:401-420 `새 보드 만들기` | aria-hidden SVG plus | "새 보드 만들기" |

지배 채널(L1845): 기능 UI 아이콘 전수 SVG — aria-hidden·viewBox 24·currentColor·strokeWidth 2.

## Goals

- 생성 트리거 3곳의 plus 마크를 SVG 단일 채널로 정렬
- 접근 이름에서 장식 기호 제거 ("핀 생성"·"새 보드")
- pill 시각(패딩·radius·색)과 토글/내비게이션 동작 불변

## Decisions

### Decision 1: SVG 채널로 정렬 (텍스트 접두 제거가 아니라 교체)

plus 마크는 add-어포던스 시각 신호로 유지 가치가 있다(3곳 모두 채택 중). 제거(대안 A)는 어포던스 손실, `<span aria-hidden>+</span>` 래핑(대안 B)은 글리프 채널 잔존으로 L1845 지배 채널(기능 UI = SVG)과 계속 어긋난다. AddToBoardButton:405 idiom을 복제해 SVG로 교체한다.

```tsx
<svg
  aria-hidden="true"
  width="14"
  height="14"
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  strokeWidth="2"
  strokeLinecap="round"
  strokeLinejoin="round"
>
  <line x1="12" y1="5" x2="12" y2="19" />
  <line x1="5" y1="12" x2="19" y2="12" />
</svg>
```

크기 14는 AddToBoardButton(text-sm 병치)과 동일값 — NavBar CTA는 text-sm, MyPageClient 버튼은 text-xs이지만 L306(아이콘 크기 위계) 상 14는 인라인 라벨 병치 크기로 기존 사용례와 일치한다. 단, MyPageClient는 text-xs(12px)라 14가 과대할 수 있어 12로 축소한다(SearchBar:144 돋보기 등 12 사용례 없음 확인 필요 → 구현 시 기존 크기 집합 {12,14,16,18,20,24} 중 확인). 기본은 NavBar 14·MyPageClient 12.

### Decision 2: flex 병치 정렬

- NavBar Link: `className`에 `inline-flex items-center gap-1.5` 추가 (Link는 inline 요소 — pill 패딩 px-4 py-2 유지).
- MyPageClient 버튼: `inline-flex items-center gap-1` 추가 (px-3 py-1.5 text-xs 유지).

gap 값은 AddToBoardButton의 gap-2(text-sm·전폭 타일)보다 컴팩트한 pill에 맞춰 축소 — 기존 코드베이스 gap-1/gap-1.5 사용례와 일치.

### Decision 3: AddToBoardButton 비접촉

이미 지배 채널 준수. 작업 범위 원칙상 미수정.

## Risks

- 시각 회귀: 텍스트 "+"(폰트 글리프)와 SVG 스트로크의 굵기/크기 차이로 pill 폭이 미세 변동 — CDP 스크린샷/치수 확인으로 검증.
- 접근 이름 변경("+ 핀 생성"→"핀 생성"): 이름을 참조하는 테스트/QA 스크립트가 있으면 갱신 필요 — grep으로 전수 확인.
