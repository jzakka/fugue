## Context

DESIGN.md L26-35는 typography scale을 의미 카테고리별로 명시한다:

- L33 `xs: 12px / 0.75rem — creator name, meta`
- L34 `2xs: 11px / 0.6875rem — timestamps, duration`
- L35 `3xs: 10px / 0.625rem — tags, category labels`

핀 상세 페이지(`apps/web/src/app/pins/[id]/page.tsx:130`)의 좌상단 미디어 타입 배지는 `getMediaTypeLabel(pin.media_type)` 헬퍼를 통해 미디어 카테고리 라벨(현재 매핑: `image → Image`, `audio → Music`, `video → Video`, 그 외 원본 키 폴백)을 노출한다. 이는 작품의 미디어 카테고리를 분류하는 라벨이므로 DESIGN.md L35의 'category labels' 매핑 대상이다. 현재 className은 `text-xs`(12px / creator name·meta 카테고리)로 어긋난다.

동일 파일 L152의 정적 태그 칩은 archive/2026-05-18-pin-detail-tag-chip-text-3xs-20260518에서 `text-3xs`로 정렬됐다. 미디어 타입 배지는 그 정렬에서 누락된 잔여 1건. 토큰 `--text-3xs: 0.625rem`은 `apps/web/src/app/globals.css` `@theme inline`에 archive/2026-05-15-text-scale-tokens-2xs-3xs에서 정의되어 즉시 사용 가능.

## Goals / Non-Goals

**Goals:**
- 핀 상세 페이지 미디어 타입 배지의 typography 토큰을 DESIGN.md L35 'tags, category labels' 카테고리(3xs)에 정렬.
- 변경 범위를 단일 단어 교체(`text-xs` → `text-3xs`)로 한정하여 effort=1 / risk=1 유지.

**Non-Goals:**
- 미디어 타입 배지의 다른 시각 속성(`px-3 py-1`, `bg-accent-subtle`, `text-accent`, `rounded-full`, `font-mono`, `font-medium`)은 변경하지 않는다.
- 다른 미디어 타입 배지 사용처는 본 변경 범위 외:
  - `apps/web/src/app/pin/new/PinCreateForm.tsx:388` — 부모 div `text-xs` 상속이라 독립 변경 시 부모 텍스트 영역 시각 회귀 위험(부모 라인은 `flex items-center gap-2 text-xs text-text-muted`로 파일명·미디어 타입·클립 시간을 묶음).
  - `apps/web/src/app/boards/[id]/page.tsx:69` — '비공개' 상태 라벨이며 'category labels' 매핑이 자의적(상태/권한 라벨은 미디어 카테고리와 별도).
- 핀 상세 페이지의 다른 typography 카테고리 정렬(creator name·timestamp 등)은 별도 후보.

## Decisions

### Decision 1: `text-xs` → `text-3xs` 단일 utility 교체

**Rationale:** DESIGN.md L35가 'tags, category labels' 카테고리에 3xs를 명시적으로 매핑하고, 토큰 `--text-3xs: 0.625rem`이 globals.css에 이미 정의되어 있어 1단어 교체만으로 명세 정렬이 완료된다. 미디어 타입(이미지/오디오/영상/텍스트)은 작품 미디어 카테고리 분류 라벨이므로 'category labels' 매핑이 직접적이며 자의적 해석이 아니다.

**Alternatives considered:**
- 미디어 타입 배지 유지(`text-xs`) → DESIGN.md L35 직접 위반. 같은 파일 태그 칩 정합 사례와도 불일치.
- 새 토큰 도입 → 이미 `--text-3xs` 존재. anti-patterns L16의 'scale 의미 덮어쓰기' 인접 위험.
- `text-[10px]` 매직값 → DESIGN.md 토큰 회피로 디자인 시스템 일관성 깨짐.
- PinCreateForm.tsx:388 동시 정렬 → 부모 div `text-xs` 상속 구조로 독립 변경 시 형제 텍스트(파일명·클립 시간) 영역 시각 회귀. 별도 후보로 분리.
- boards/[id]/page.tsx:69 '비공개' 동시 정렬 → 'category labels' 매핑이 자의적(상태 라벨). 별도 후보 평가.

### Decision 2: 변경 위치를 라인 130 단일 className 토큰 단어로 한정

**Rationale:** anti-patterns L16이 "신규 토큰 추가와 기존 Tailwind 기본 클래스 의미 덮어쓰기를 단일 항목으로 묶으면 effort 추정이 무너진다"를 명시. 본 변경은 신규 토큰 추가가 아니라 기존 토큰 사용처 1곳을 명세 카테고리로 옮기는 정렬이라 anti-pattern과 구조가 다르고 회귀 범위가 단일 화면 단일 요소로 한정된다.

**Alternatives considered:**
- 핀 상세 페이지 전체 typography 일제 점검 → 단발 정렬 사이클 범위(effort=1) 초과. 별도 후보로 분리.

## Risks / Trade-offs

- [Risk] 미디어 타입 라벨 글자가 작아져(12px → 10px) 가독성 저하 우려 → DESIGN.md L35가 3xs(10px)를 'tags, category labels' 카테고리에 직접 명시하고 있어 명세 결정. 가독성 정책 결정은 디자인 트랙 루프 범위 밖.
- [Risk] 미디어 타입 배지 너비가 줄어들어 좌상단 정렬 점프 → 배지는 `inline-block`이고 `px-3 py-1` padding이 유지되므로 박스 크기 변동은 글자 폭에 의한 미세 차이만 발생. 상위 컨테이너(`<div className="p-6 sm:p-8 space-y-5">`) 흐름 영향 없음.
- [Trade-off] PinCreateForm.tsx:388·boards/[id]/page.tsx:69의 유사 배지 컨텍스트는 정렬 잔여 → 본 변경 범위 외 명시. 각각 별도 후보 평가 필요.

## Migration Plan

1. `apps/web/src/app/pins/[id]/page.tsx:130` className `text-xs` → `text-3xs` 교체.
2. Next.js dev 서버에서 `/pins/<id>` 페이지 진입, 미디어 타입 배지 글자 크기 시각 확인.
3. 회귀 영역: 핀 상세 페이지 1화면 좌상단 배지. 다른 화면 영향 없음(`pins/[id]/page.tsx` 단일 파일 단일 라인).
4. Rollback: 단일 커밋 git revert.
