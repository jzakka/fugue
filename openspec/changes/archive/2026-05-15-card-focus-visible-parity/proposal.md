# 카드 5종에 focus-visible: hover state 미러링 추가

## 배경

DESIGN.md L86은 카드의 hover state를 다음과 같이 명시한다:

> Hover state: translateY(-2px) + box-shadow 확대 + accent border. 150ms ease.

루프 정체성(prompts/loop-design.md L7)은 "접근성 (대비, 키보드 포커스, aria)"을 in-scope로 명시한다. 5종 카드는 모두 `<Link>`로 키보드 포커스가 가능하지만 hover className만 정의돼 있고 `focus-visible:` 대응이 한 곳도 없다:

- `apps/web/src/components/feed/PinCard.tsx:146` — 핀 카드 (Link)
- `apps/web/src/app/search/SearchClient.tsx:313` — 검색 결과 크리에이터 카드 (Link)
- `apps/web/src/app/search/SearchClient.tsx:351` — 검색 결과 보드 카드 (Link)
- `apps/web/src/components/board/BoardGrid.tsx:22` — 보드 그리드 (Link wrapper, translate만; shadow+border는 BoardCover에)
- `apps/web/src/components/profile/MyPageClient.tsx:129` — 마이페이지 보드 그리드 (동일 패턴)
- `apps/web/src/components/board/BoardCover.tsx:6, :30` — BoardCover 두 분기 (group-hover로 shadow+border)

키보드 사용자는 디자인 의도(translateY + shadow + accent border)를 받지 못하고 브라우저 기본 outline만 본다.

## 변경 범위

`apps/web/` 안 5개 파일, 7개 className에 `focus-visible:` 또는 `group-focus-visible:` 접두사 변형 추가.

### Pattern A — 동일 요소에 hover 3효과 (PinCard, SearchClient×2)

기존 hover 3효과 옆에 같은 효과 `focus-visible:` 버전 + `focus-visible:outline-none` (기본 브라우저 outline은 새 시각으로 대체) 추가:

```
+ focus-visible:-translate-y-0.5
+ focus-visible:shadow-[0_8px_32px_rgba(0,0,0,0.3)]
+ focus-visible:border-accent
+ focus-visible:outline-none
```

대상:
1. `apps/web/src/components/feed/PinCard.tsx:146`
2. `apps/web/src/app/search/SearchClient.tsx:313`
3. `apps/web/src/app/search/SearchClient.tsx:351`

### Pattern B — Link/BoardCover 분리 (BoardGrid, MyPageClient + BoardCover×2)

Link에는 translate + outline-none:

```
+ focus-visible:-translate-y-0.5
+ focus-visible:outline-none
```

BoardCover 두 분기(L6, L30)에는 group-focus-visible 시 shadow+border:

```
+ group-focus-visible:border-accent
+ group-focus-visible:shadow-[0_8px_32px_rgba(0,0,0,0.3)]
```

대상:
4. `apps/web/src/components/board/BoardGrid.tsx:22`
5. `apps/web/src/components/profile/MyPageClient.tsx:129`
6. `apps/web/src/components/board/BoardCover.tsx:6`
7. `apps/web/src/components/board/BoardCover.tsx:30`

## 사용자 영향

- **키보드 사용자**: 5종 카드에 Tab으로 포커스 이동 시 디자인 의도와 동일한 시각 피드백(translateY + shadow + accent border)을 받음.
- **마우스 사용자**: `:focus-visible`는 키보드 포커스만 트리거하므로 클릭 시 시각 회귀 없음(Chrome/Edge/Safari/Firefox 모두 동일 휴리스틱).
- **터치 사용자**: 동일하게 focus-visible 미발동.

## anti-pattern 검토

- L15(Tailwind 기본 토큰 의미 덮어쓰기): 해당 없음. 토큰 변경 0.
- L16(DESIGN.md radius 등급 매핑 모호): 해당 없음. radius 영역 변경 0.

## 사용자 결정 위반 검토

decision-log 최근 10개 검토:
- `2026-05-15-add-board-card-hover-state` (사이클 11): 보드 카드 hover state 추가. 본 변경은 동일 트랙의 focus 대응 보강. 충돌 없음.
- `2026-05-15-search-dropdown-keyboard-a11y` (사이클 18): SearchBar 드롭다운 키보드 a11y. 본 변경은 카드 컴포넌트 영역으로 분리. 충돌 없음.
- `2026-05-15-icon-only-button-aria-label` (사이클 28): 버튼 aria-label. 본 변경은 카드 focus 영역. 충돌 없음.

## 롤백 절차

5개 파일에서 추가한 className 토큰들 제거(총 7곳 className).
