## Why

DESIGN.md L33-34는 typography scale을 'xs: 12px (creator name, meta)'와 '2xs: 11px (timestamps, duration)'로 두 단계로 분리해 meta와 timestamps에 다른 크기를 명시한다. 그러나 `apps/web/src/app/search/SearchClient.tsx:328`의 크리에이터 검색 카드에서 가입일(`new Date(creator.created_at).toLocaleDateString('ko-KR')`)을 표시할 때 className `text-xs text-text-dim font-mono`로 xs(12px / meta 카테고리) 매핑을 사용한다. 가입일은 DESIGN.md L34 'timestamps' 카테고리의 직접 대응이므로 2xs(11px) 매핑이어야 한다. 사이클 archive/2026-05-15-text-scale-tokens-2xs-3xs에서 `--text-2xs: 0.6875rem` 토큰이 globals.css `@theme inline`에 이미 정의되고 PinCard/SearchBar 매직값 `text-[10px]`를 `text-3xs`로 회수했으나, 가입일 timestamp 매핑은 미회수 잔여로 남아 있다.

## What Changes

- `apps/web/src/app/search/SearchClient.tsx:328` className에서 `text-xs` → `text-2xs` 단어 1개 교체로 글자 크기 12px → 11px 정렬.
- 매핑 범위는 가입일(`creator.created_at`) timestamp 표시 1곳으로 한정. 동일 카드의 크리에이터 닉네임(L325 `text-sm font-semibold`)은 DESIGN.md L32 secondary text 카테고리라 본 변경 범위 밖.
- 시각 변화: 가입일 글자 크기 12px → 11px(1px 감소). font-mono / text-text-dim 그대로 유지.
- 비포함: 다른 가능한 timestamp/duration 표시 위치는 현재 코드베이스에 없음(grep `created_at` apps/web 결과 SearchClient 단일 사이트). VideoTrimModal L164/L222는 이미 `text-2xs`로 정렬 완료.

## Capabilities

### New Capabilities
- `design-tokens`: DESIGN.md의 텍스트 카테고리(Display·Body·UI/Labels·Data/Tags·Timestamps 등)별 weight/size를 UI 컴포넌트에 일관 적용하는 토큰 매핑 규칙. 본 변경에서는 timestamp 카테고리(L34 '2xs (11px): timestamps, duration') 매핑을 코드 SSoT에 도입한다.

### Modified Capabilities
- (없음): 외부 관찰 가능한 행위(이벤트·API 응답·라우팅·상호작용 결과)는 변동 없다. DESIGN.md 명시 시각 토큰을 코드에 반영하는 변경.

## Impact

- 영향 파일 1개(`apps/web/src/app/search/SearchClient.tsx`) 1라인.
- 영향 페이지: `/search`(검색 페이지) 크리에이터 결과 카드의 가입일 표시.
- API·DB·라우팅 변경 없음.
- 의존성 변경 없음(`--text-2xs` 토큰은 globals.css @theme inline에 이미 정의).
- 롤백 절차: 단일 커밋 `git revert`.
