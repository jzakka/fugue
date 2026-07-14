# Design: pinsgrid-filter-active-fill-vocab

## Context

PinsGrid.tsx:104-119 미디어타입 필터 칩(aria-pressed 토글 버튼 4개)만 활성 채움을 `bg-accent text-white` 로 렌더한다. 동일 아키타입(타입/카테고리 뷰-필터) 4표면의 확립 어휘는 반전 채움 `bg-text-primary text-bg` 이고, 액센트 채움은 태그 칩 role(DESIGN.md L41)에 배정되어 있다. FieldFilter.tsx:31-47 은 동일 MEDIA_TYPES 라벨의 정확한 비교쌍이다.

## Goals / Non-Goals

**Goals:**
- PinsGrid 필터 칩의 활성/hover/focus/사이징 어휘를 FieldFilter(뷰-필터 role)와 정합화.

**Non-Goals:**
- 비활성 base 색 변경(`bg-surface` 유지 — PinCreateForm 카테고리 필터와 동일한 role 내 확립 변형).
- 필터 로직·API 파라미터·마크업 구조·다른 표면(태그 칩 등) 변경.
- 공용 FilterChip 컴포넌트 추출(상태 블록 컴포넌트화 축은 anti-patterns L1871 에서 저작-채널로 기판정).

## Decisions

1. **활성 채움 = 반전(`bg-text-primary text-bg`)**: 뷰-필터 4표면의 확립 어휘. 액센트 유지 대안은 DESIGN.md L38 Restrained 원칙(액센트 절제)과 L41 role 배분에 역행.
2. **hover/focus-visible border = `border-text-muted`**: FieldFilter·SearchClient 탭의 확립형. `hover:border-accent` 는 뷰-필터 표면 중 PinsGrid 유일.
3. **`py-1.5` + `font-medium`**: FieldFilter chip 사이징/웨이트와 일치시켜 동종 아키타입의 시각 동형성 복원.
4. **비활성 base `bg-surface border border-border` 유지**: FieldFilter 는 `bg-transparent` 지만 PinCreateForm:586·:600 이 `bg-surface` 변형을 확립. 보수 원칙(기존 시각 최소 변경)에 따라 base 는 건드리지 않고 어휘 침범 축(활성 채움·hover 액센트)만 교정.

## Risks / Trade-offs

- [프로필 페이지 사용자에게 활성 칩 색이 주황→반전으로 변경] → 의도된 정합화. FieldFilter 와 동일 시각이므로 학습 비용 없음. revert 1회 롤백 가능.
- [py-2→py-1.5 로 칩 높이 2px 감소] → FieldFilter 와 동형화가 목적. 터치 타겟은 px-4 + 텍스트로 충분 유지.

## Migration Plan

단일 커밋, className 문자열 교체만. 배포 특이사항 없음. 롤백 = revert.
