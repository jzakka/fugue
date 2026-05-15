# 카드 hover/focus-visible 그림자 토큰화

## 요약

카드 hover/focus-visible 그림자가 `shadow-[0_8px_32px_rgba(0,0,0,0.3)]` 매직값으로
5개 파일 10곳에 중복돼 있다. `--shadow-card-hover` 토큰으로 SSoT화한다.

## 배경

DESIGN.md L86 — "Hover state: translateY(-2px) + box-shadow 확대 + accent border. 150ms ease." (Card System 섹션).

DESIGN.md는 카드 hover 시 box-shadow 확대를 명시하지만 구체적인 shadow 값은
정의하지 않는다. 사이클 28(`archive/2026-05-15-card-focus-visible-parity`)에서
hover/focus-visible 패리티를 맞추는 과정에서 동일 매직값 `0 8px 32px rgba(0,0,0,0.3)`이
5개 카드 파일에 hover·focus-visible 각 2회씩 박혀 총 10곳에 산재됐다.

`apps/web/src/app/globals.css` `@theme inline` 블록에 `--shadow-*` 토큰 정의가 없어
디자인 시스템 SSoT가 깨진 상태다. 향후 카드 hover 그림자 톤 조정 시 5개 파일을
동시 수정해야 한다.

## 변경 범위

### 추가
- `apps/web/src/app/globals.css` `@theme inline` 블록 끝에 토큰 한 줄 추가:
  ```css
  --shadow-card-hover: 0 8px 32px rgba(0, 0, 0, 0.3);
  ```
  Tailwind v4의 `@theme` shadow 토큰 명명 규칙 `--shadow-<name>`을 따라
  `shadow-card-hover` 유틸리티 클래스가 자동 활성화된다.

### 치환 (10건)
- `apps/web/src/components/feed/PinCard.tsx:146` (hover 1, focus-visible 1)
- `apps/web/src/app/search/SearchClient.tsx:313` (hover 1, focus-visible 1)
- `apps/web/src/app/search/SearchClient.tsx:351` (hover 1, focus-visible 1)
- `apps/web/src/components/board/BoardCover.tsx:6` (group-hover 1, group-focus-visible 1)
- `apps/web/src/components/board/BoardCover.tsx:30` (group-hover 1, group-focus-visible 1)

각 위치에서 `shadow-[0_8px_32px_rgba(0,0,0,0.3)]` → `shadow-card-hover` 단순 치환.
`hover:`, `focus-visible:`, `group-hover:`, `group-focus-visible:` 프리픽스는 유지.

## 영향

### 시각
변경 전후 동일한 box-shadow 값(`0 8px 32px rgba(0,0,0,0.3)`)을 가리키므로
사용자 시각 변경 0.

### 디자인 시스템
- SSoT 확보: 카드 hover 그림자 톤 조정이 globals.css 1곳 수정으로 완결.
- DESIGN.md L86 의도("box-shadow 확대")가 토큰명으로 자기 문서화.

### 회귀 위험
없음. `shadow-card-hover`는 신규 토큰명이고 Tailwind v4 기본 shadow-* 유틸리티
(`shadow-sm`, `shadow-md`, `shadow-lg`, `shadow-xl`, `shadow-2xl`)와 충돌하지 않는다.
사전 grep으로 `shadow-card-hover` 0건 확인 가능.

## 롤백

`git revert` 한 커밋으로 토큰 추가 + 5개 파일 치환을 동시에 되돌린다.

## DESIGN.md 인용

- L86 "Hover state: translateY(-2px) + box-shadow 확대 + accent border. 150ms ease."

## 출처

- `.fugue/backlog-design.yaml` — `design-20260515-card-hover-shadow-magic-value` (score 4.0)
