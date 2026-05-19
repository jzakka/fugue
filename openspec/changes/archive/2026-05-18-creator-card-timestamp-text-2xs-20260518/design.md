## Context

DESIGN.md L26-35는 typography scale 9단계를 정의하며 각 단계에 카테고리를 명시한다(3xl: hero, 2xl: page title, …, xs: creator name + meta, 2xs: timestamps + duration, 3xs: tags + category labels). 사이클 archive/2026-05-15-text-scale-tokens-2xs-3xs에서 `--text-2xs: 0.6875rem` 토큰을 globals.css `@theme inline`에 추가하고 PinCard/SearchBar 매직값 `text-[10px]`를 `text-3xs`로 회수했다.

현재 상태:
- `apps/web/src/app/globals.css:58-59`: `--text-2xs: 0.6875rem; --text-3xs: 0.625rem;` 토큰 정의 완료(@theme inline).
- 코드 사용처: `text-2xs` 2건(VideoTrimModal L164/L222 — 트림 시간 표시), `text-3xs` 2건(PinCard L197 / SearchBar L293 — 태그 chip).
- 잔여 갭: `apps/web/src/app/search/SearchClient.tsx:328`의 크리에이터 가입일 표시가 `text-xs`(12px / meta 카테고리)로 매핑. DESIGN.md L34는 timestamp을 2xs(11px)로 명시.

## Goals / Non-Goals

**Goals:**
- DESIGN.md L34 'timestamps' 카테고리 매핑을 코드 SSoT에 반영한다.
- 크리에이터 가입일 표시 1곳의 className `text-xs` → `text-2xs`로 1단어 교체한다.

**Non-Goals:**
- 다른 timestamp/duration 표시 위치를 새로 만들거나 변경하지 않는다(현재 가입일 외 timestamp 표시 사이트 없음, VideoTrimModal은 이미 정렬됨).
- 크리에이터 닉네임(`text-sm font-semibold`, DESIGN.md L32 secondary text 카테고리)은 본 변경 범위 밖.
- 토큰 정의 변경 없음(globals.css는 손대지 않음).
- 광의 'timestamps' 해석(created_at 외 모든 날짜/시간 데이터에 2xs 적용)은 자의성 회피를 위해 본 변경 범위 밖.

## Decisions

### Decision 1: `text-xs` → `text-2xs` 1단어 교체로 한정

크리에이터 카드 가입일의 className은 현재 `text-xs text-text-dim font-mono`이다. `text-xs` 단어 1개만 `text-2xs`로 교체한다.

**대안 검토:**
- (A) 검색 카드 전체 typography 위계 재설계(닉네임 sm → xs, 가입일 xs → 2xs 등): 위계 차이 강화 가능하지만 변경 폭 증가·시각 회귀 위험·DESIGN.md가 검색 카드 위계를 직접 명시하지 않아 자의적 해석. 본 변경 범위에서 제외.
- (B) `text-xs` → 매직값 `text-[11px]`: 토큰화 정착 패턴(archive/2026-05-15-text-scale-tokens-2xs-3xs)을 역행. anti-pattern L16(매직값) 직격.

**선택 근거:** `text-2xs` 토큰이 이미 정의되어 있고, DESIGN.md L34 'timestamps' 카테고리에 직접 대응. 변경 폭 1라인 1단어.

### Decision 2: 매핑 범위를 `created_at` 표시 1곳으로 한정

현재 코드베이스에서 timestamp을 표시하는 사이트는 SearchClient L328-332 한 곳뿐이다(grep `created_at|toLocaleDateString` apps/web/src 결과 SearchClient 단일). VideoTrimModal L164/L222의 트림 시간 표시는 이미 `text-2xs`로 정렬됨.

**대안 검토:**
- "Pin 상세 페이지에 가입일 표시 추가" 같은 신규 표시 도입: 본 변경은 매핑 정렬이지 신규 기능 도입이 아님. 별도 후보로 분리.
- '모든 날짜/시간 데이터'로 광의 적용: created_at 외에 표시 데이터가 없어 적용 대상 없음. 광의 해석 시 anti-pattern L16 자의성 위험.

**선택 근거:** 매핑 잔여 회수에 한정하면 명확한 위반-수정 매핑이 되어 자체 리뷰 reject 조건 #2(자의적 해석)를 회피한다.

## Risks / Trade-offs

- [Risk] 가입일 1px 축소(12px → 11px)가 가독성에 영향 → Mitigation: DESIGN.md L34 명시값이 11px이고, 동일 카드의 닉네임(`text-sm` = 13px)과 위계 차이가 2px 더 벌어져 정보 위계 명확화. font-mono 유지로 tabular-nums 정렬 영향 없음.
- [Risk] anti-pattern L15(Tailwind 기본 의미 덮어쓰기) 위반 가능성 → Mitigation: `text-2xs`는 Tailwind 기본 스케일에 없는 신규 토큰(archive/2026-05-15-text-scale-tokens-2xs-3xs에서 추가). 기본 의미 덮어쓰기 아님.
- [Risk] anti-pattern L16(자의적 등급 매핑) 위반 가능성 → Mitigation: 가입일 = timestamp 매핑은 DESIGN.md L34 'timestamps' 직접 명시. 자의적 해석 아님.

## Migration Plan

1. `apps/web/src/app/search/SearchClient.tsx:328`의 className에서 `text-xs` → `text-2xs` 교체.
2. 사후 검증: grep `text-2xs` apps/web/src --include='*.tsx' 결과 3건 확인(이전 2건 + 신규 1건). grep `created_at` 사용처에서 `text-xs` 사용 0건 확인.
3. 롤백: 단일 커밋 `git revert`.

## Open Questions

없음. DESIGN.md L34 명시 매핑이 명확하고 사용처 1곳이라 모호성 없음.
