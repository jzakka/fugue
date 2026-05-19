# 아이콘 전용 버튼 4곳에 aria-label 추가

## 배경

루프 정체성(prompts/loop-design.md L7)에 "접근성"이 명시 in-scope다. WCAG 4.1.2 (Name, Role, Value) 및 WAI-ARIA 권고는 모든 인터랙티브 컨트롤이 접근 이름(accessible name)을 갖도록 요구한다. `apps/web/src` 안에는 visible text 라벨 없이 SVG 또는 Unicode 글리프만 가진 버튼이 4곳 있다:

1. `apps/web/src/components/board/AddToBoardButton.tsx:210-227` — "보드에 추가" 모달의 헤더 닫기 X 버튼 (SVG only)
2. `apps/web/src/components/nav/SearchBar.tsx:216-234` — 헤더 검색 드롭다운에서 최근 검색 항목 hover 시 노출되는 삭제 X 버튼 (SVG only)
3. `apps/web/src/components/feed/PinCard.tsx:63-66` — 오디오 핀 카드의 ▶ 재생 버튼 (Unicode 글리프만, 접근 이름 = "BLACK RIGHT-POINTING TRIANGLE")
4. `apps/web/src/components/feed/PinCard.tsx:104-132` — 핀 카드 푸터의 ExternalLinkIcon (SVG only, `title="원본 보기"` 폴백만 존재)

스크린리더(VoiceOver/NVDA)로 포커스를 가져가면 (1)/(2)는 "button", (3)은 글리프 이름, (4)는 일부 AT에서만 title 폴백을 읽는다.

직전 사이클 18 archive change `2026-05-15-search-dropdown-keyboard-a11y`가 SearchBar 드롭다운 5종 아이템 마크업을 button/Link로 정렬했지만, 본 X 삭제 버튼은 button-in-button 회피 구조의 sibling으로 분리됐을 뿐 aria-label은 부여되지 않았다.

## 변경 범위

`apps/web/` 안 4개 파일, 각 1단어(`aria-label="..."`) 추가:

1. **`apps/web/src/components/board/AddToBoardButton.tsx:210`** — 모달 닫기 `<button>`에 `aria-label="닫기"` 추가.
2. **`apps/web/src/components/nav/SearchBar.tsx:216`** — 최근 검색 삭제 `<button>`에 `aria-label="최근 검색에서 제거"` 추가.
3. **`apps/web/src/components/feed/PinCard.tsx:64`** — 오디오 카드 재생 `<button>`에 `aria-label="재생"` 추가.
4. **`apps/web/src/components/feed/PinCard.tsx:106`** — `ExternalLinkIcon` 컴포넌트의 `<button>`에 `aria-label="원본 보기"` 추가 (`title`도 유지 — sighted user에게 hover tooltip 제공).

## 사용자 영향

- 시각·행동 변경 없음. visible text/스타일/onClick 모두 동일.
- 스크린리더 사용자: 4개 버튼의 정체가 명시적으로 안내됨. 키보드만 사용하는 sighted user에게도 일관된 hover/focus tooltip(ExternalLinkIcon `title`).
- 회귀 위험: aria-label은 시각/레이아웃에 영향이 없는 순수 보조 속성. 다른 컴포넌트 영향 없음.

## anti-pattern L15/L16 검토

- L15(Tailwind 기본 토큰 의미 덮어쓰기): 해당 없음. 토큰 변경 0.
- L16(DESIGN.md radius 등급 매핑 모호): 해당 없음. DESIGN.md radius 영역 변경 0.

## 사용자 결정 위반 검토

decision-log 최근 10개 검토 결과 본 변경과 충돌하는 사용자 결정 없음. 사이클 18(`2026-05-15-search-dropdown-keyboard-a11y`)이 동일 a11y 트랙에서 디자인 루프 범위(`apps/web/`)임을 이미 확인.

## 롤백 절차

각 파일에서 추가한 `aria-label="..."` 한 단어씩 제거(총 4곳).
