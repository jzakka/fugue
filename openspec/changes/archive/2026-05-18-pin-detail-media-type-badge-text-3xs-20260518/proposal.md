## Why

핀 상세 페이지 좌상단의 미디어 타입 배지(`getMediaTypeLabel(pin.media_type)` 반환값 — 현재 `Image`/`Music`/`Video`)는 작품의 미디어 카테고리를 노출하는 라벨이다. DESIGN.md L35는 'tags, category labels' 카테고리를 3xs(10px)에 매핑하지만, 현재 이 배지는 12px(creator name·meta 카테고리)로 렌더링되어 의미 카테고리와 시각 위계가 어긋난다. 같은 파일의 태그 칩(archive/2026-05-18-pin-detail-tag-chip-text-3xs-20260518) 정렬에서 누락된 잔여 1건.

## What Changes

- `apps/web/src/app/pins/[id]/page.tsx:130` 미디어 타입 배지 `<span>` className `text-xs` → `text-3xs` 1단어 교체
- font-mono / text-accent / bg-accent-subtle / rounded-full / px-3 py-1 / font-medium 모두 유지
- 표시되는 미디어 카테고리 라벨 글자 크기 12px → 10px (2px 감소)

## Capabilities

### New Capabilities
- `design-tokens`: DESIGN.md typography scale(L26-35) 'tags, category labels' 카테고리가 코드 SSoT 내에서 의미 카테고리별로 일관된 시각 위계로 렌더링되도록 보장하는 행위 계약. archive/2026-05-18-pin-detail-tag-chip-text-3xs-20260518·archive/2026-05-18-creator-card-timestamp-text-2xs-20260518과 동일 capability에 누적.

### Modified Capabilities
(none)

## Impact

- 파일: `apps/web/src/app/pins/[id]/page.tsx` 1라인
- 시각 변화: 핀 상세 페이지 좌상단 미디어 카테고리 라벨 글자 크기 12px → 10px. font-mono / text-accent / bg-accent-subtle / rounded-full / px-3 py-1 / font-medium 모두 유지
- 다른 미디어 타입 배지 사용처는 본 변경 범위 밖:
  - `apps/web/src/app/pin/new/PinCreateForm.tsx:388` — 부모 div `text-xs` 상속이라 독립 변경 시 부모 영역 회귀 위험
  - `apps/web/src/app/boards/[id]/page.tsx:69` — '비공개' 상태 라벨이라 'category labels' 매핑이 자의적
- 토큰: `--text-3xs: 0.625rem`는 `apps/web/src/app/globals.css` `@theme inline`에 이미 정의되어 있어 신규 토큰 추가 불필요(archive/2026-05-15-text-scale-tokens-2xs-3xs)
- 의존성: 없음. apps/api / DB / 서드파티 무관
- 롤백: 단일 커밋 git revert
