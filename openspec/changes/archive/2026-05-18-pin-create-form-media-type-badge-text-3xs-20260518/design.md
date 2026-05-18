## Context

DESIGN.md L26-35는 타이포 스케일 9단계를 정의하며 각 단계에 의미 카테고리를 직접 매핑한다.

- L33 — `xs: 12px / 0.75rem (creator name, meta)`
- L34 — `2xs: 11px / 0.6875rem (timestamps, duration)`
- L35 — `3xs: 10px / 0.625rem (tags, category labels)`

`apps/web/src/app/pin/new/PinCreateForm.tsx`는 파일 업로드 후 메타 정보 영역에 (1) 파일명, (2) 미디어 타입 배지, (3) trim 정보, (4) 원본/최적화 사이즈 비교를 부모 div 한 줄에 표시한다. 부모 L386 div는 `text-xs text-text-muted flex items-center gap-2`로 자식들이 text-xs(12px / 'meta' 카테고리)를 상속받는 구조다.

L388 미디어 타입 배지 `<span>`은 자체 text 크기 utility 없이 부모 text-xs를 상속한다. 표시 데이터인 `mediaType` 변수(L289-297)는 file MIME prefix에 따라 "이미지/오디오/비디오"를 반환하는 한국어 카테고리 라벨이므로, 의미상 DESIGN.md L35 'category labels' 카테고리에 정확히 매핑된다.

archive/2026-05-15-text-scale-tokens-2xs-3xs에서 globals.css `@theme inline` 블록에 `--text-2xs: 0.6875rem`/`--text-3xs: 0.625rem` 토큰이 정의되었고 Tailwind v4가 `text-2xs`/`text-3xs` utility를 자동 생성한다. archive/2026-05-18-pin-detail-tag-chip-text-3xs-20260518가 `pins/[id]/page.tsx:152` 정적 태그 칩을, archive/2026-05-18-pin-detail-media-type-badge-text-3xs-20260518가 같은 파일 L130 미디어 타입 배지를 3xs로 정렬한 후속 잔여 1건이 본 변경 대상이다.

## Goals / Non-Goals

**Goals:**
- PinCreateForm 미디어 타입 배지가 DESIGN.md L35 'category labels' 카테고리(3xs / 10px)로 렌더링되도록 자식 utility 정렬
- 부모 div utility 미수정으로 같은 부모 안 다른 자식(파일명·trim 정보·사이즈 비교) 영역 회귀 0 보장
- 미디어 카테고리 라벨이 코드베이스 전체에서 일관되게 3xs로 정렬되는 SSoT 정합 강화 (pins/[id]:130 + pin/new:388)

**Non-Goals:**
- 부모 L386 div의 `text-xs` utility 변경 (다른 자식에 광범위 회귀 발생)
- 같은 부모 안 다른 자식 텍스트(파일명·trim·사이즈) 카테고리 재매핑
- `boards/[id]/page.tsx:69` '비공개' 상태 라벨 (state label이 'category labels' 매핑인지 자의적 — 별도 후보 검토)
- PinCreateForm 내 다른 태그 픽커(L523/L579 인터랙티브 button 태그) 카테고리 재매핑 (UI labels 별도 카테고리)
- DESIGN.md 스케일 정의 자체 수정
- 토큰 추가 (이미 정의됨)

## Decisions

### Decision 1: 자식 span에 `text-3xs` 1단어 추가 (부모 utility 미수정)

L388 자식 `<span>` className 끝에 `text-3xs` 1단어 추가하여 자식 자체 utility로 부모 상속을 끊는다. 부모 L386 div의 `text-xs text-text-muted flex items-center gap-2`는 미수정.

**Why:**
- DESIGN.md L35가 'category labels'를 3xs로 직접 매핑하며 자의적 해석 여지가 없음
- 자식 span에 명시한 `text-3xs`가 부모 상속 `text-xs`를 정상적으로 override (CSS specificity는 동일하나 cascade 순서로 자식이 우선)
- archive/2026-05-15-text-scale-tokens-2xs-3xs에서 `--text-3xs: 0.625rem` 토큰 정의 완료, Tailwind v4 자동 생성된 `text-3xs` utility 즉시 사용 가능
- 같은 file에서 같은 패턴이 `pins/[id]/page.tsx:130`(archive 정렬됨)·`PinCard.tsx:197`(archive 정렬됨)·`SearchBar.tsx:293`(archive 정렬됨)로 이미 적용된 SSoT 정합 정렬

**Alternatives considered:**

- **부모 L386 div의 `text-xs`를 제거하고 자식들에 명시적 utility를 분배**: 부모 영역의 파일명·trim·사이즈 자식이 모두 영향받아 광범위 회귀 위험. 본 변경 범위 초과.
- **별도 `font-mono text-3xs` 조합 컴포넌트 추출**: 미디어 타입 배지 사용처가 코드베이스 4건(pins/[id]:130 archive 정렬됨·PinCreateForm:388·boards/[id]:69 별도 카테고리·기타) 정도로 한정적이고 패턴 추출 필요성 미흡. 본 변경 범위 초과.
- **DESIGN.md L35 카테고리 매핑 확장**: 코드 정렬이 아닌 스펙 수정으로 디자인 트랙 루프 범위 밖.

### Decision 2: 변경 범위를 L388 한 줄로 한정

L388 `<span>` 한 줄만 변경한다. 같은 파일 안 다른 미디어 타입 배지 사용처(L523/L579 태그 픽커 인터랙티브 button)나 다른 자식 텍스트(파일명·trim·사이즈)는 본 변경 범위에서 제외.

**Why:**
- L523/L579는 사용자 인터랙션이 있는 button 태그 픽커로 'category labels' 정적 배지와 다른 UI labels 카테고리에 속함
- 같은 부모 div의 자식 텍스트(파일명·trim·사이즈)는 DESIGN.md 명세에 직접 매핑이 없는 영역으로 자의적 재매핑 위험
- 변경 범위 최소화로 회귀 검증을 코드 진단(diff)만으로 완료 가능
- 단발 정렬 사이클 패턴 유지 (archive/2026-05-18-creator-card-timestamp-text-2xs·archive/2026-05-18-pin-detail-tag-chip-text-3xs와 동일 단위)

## Risks / Trade-offs

- **[Risk] dev 서버 시각 검증 불가**: Ralph 루프 환경에 `apps/web/node_modules` 미설치로 `npm run dev` 실행 불가 → 시각 회귀 100% 보장 못 함.
  - **Mitigation**: 변경이 1라인 utility 추가에 한정되고, archive/2026-05-15-text-scale-tokens-2xs-3xs에서 동일 토큰을 다른 컴포넌트(PinCard L197·SearchBar L293)에 적용해 정합성 확인된 정의 토큰을 재사용. 코드 검증(grep으로 L388 `text-3xs` 1건 확인 + 다른 utility 모두 유지 확인 + 부모 L386 utility 미수정 확인)으로 정합성 검증.

- **[Risk] 부모 영역 자식 회귀**: 부모 utility를 수정하지 않더라도 자식의 명시적 `text-3xs`가 flex 컨테이너 정렬에 영향을 주는 시나리오는 가능.
  - **Mitigation**: `flex items-center` 정렬 기준이 line-height 기반이라 자식 글자 크기 변경(12px → 10px)은 line-height 따라 함께 축소되어 컨테이너 정렬에 영향 없음. archive/2026-05-18-pin-detail-media-type-badge-text-3xs-20260518에서 동일 utility 변경이 시각 회귀 없이 적용된 선행 사례.

- **[Trade-off] DESIGN.md 'category labels' 카테고리 해석**: 'category labels'를 '작품의 미디어 카테고리 라벨'(현재 매핑)로 보지 않고 다른 의미(예: UI 섹션 라벨)로 본다면 매핑이 달라질 수 있음.
  - **Mitigation**: 'category labels' = '작품 미디어 카테고리 라벨' 해석은 archive/2026-05-15-text-scale-tokens-2xs-3xs(`PinCard.tsx:197` 정적 태그 칩)·archive/2026-05-18-pin-detail-tag-chip-text-3xs-20260518(`pins/[id]/page.tsx:152` 정적 태그 칩)·archive/2026-05-18-pin-detail-media-type-badge-text-3xs-20260518(`pins/[id]/page.tsx:130` 미디어 타입 배지)에서 일관 적용된 선례. 본 변경은 그 패턴의 후속 잔여 1건 정렬.

## Migration Plan

**적용 절차:**
1. `apps/web/src/app/pin/new/PinCreateForm.tsx:388` 자식 `<span>` className 끝에 `text-3xs` 1단어 추가
2. 다른 utility(font-mono / px-2 / py-0.5 / bg-accent-subtle / text-accent / rounded-full) 변경 없음 코드 검증 (grep)
3. 부모 L386 div utility 미수정 코드 검증 (grep)
4. 같은 부모 div 안 다른 자식 텍스트(L387 파일명 / L391-395 trim / L396-402 사이즈) 미수정 코드 검증 (grep)
5. archive 처리 후 decision-log에 1-3줄 추가

**롤백 절차:**
- 단일 라인 변경이므로 `git revert <commit>`로 1라인 되돌리기

**검증:**
- `grep -n 'text-3xs' apps/web/src/app/pin/new/PinCreateForm.tsx` → L388 1건 매칭 (다른 라인 변화 없음)
- `grep -n 'bg-accent-subtle text-accent rounded-full' apps/web/src/app/pin/new/PinCreateForm.tsx` → L388 1건 매칭 (다른 utility 유지)
- `git diff apps/web/src/app/pin/new/PinCreateForm.tsx` → ±1 라인, L388 한 줄만 변경 확인
- dev 서버 시각 검증(`npm run dev` → 핀 생성 페이지 파일 업로드 후 미디어 타입 배지 글자 크기 10px 확인)은 Ralph 루프 환경 제약으로 미수행
