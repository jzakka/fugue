## Context

메인 피드 페이지는 현재 분야(field) 필터만 제공한다. `FieldFilter` 컴포넌트가 URL 쿼리 파라미터 `?field=`로 동작하며, `FeedContainer`가 `fetchPins({ field })` 호출로 필터링된 결과를 보여준다.

백엔드 `GET /api/pins?tags=tag1,tag2`는 이미 태그 overlap 필터를 지원하고, 프론트엔드 `fetchPins`에도 `tags` 파라미터가 존재한다. 빠져 있는 것은 (1) 어떤 태그를 보여줄지 결정하는 API와 (2) 사용자가 태그를 선택하는 UI다.

## Goals / Non-Goals

**Goals:**
- 메인 피드에서 인기 태그 기반 필터링 UI 제공
- 인기 태그 목록을 DB에서 집계하는 API 엔드포인트 제공
- 태그 필터 상태를 URL 쿼리 파라미터로 관리 (공유/북마크 가능)
- 분야 필터와 태그 필터를 조합하여 사용 가능

**Non-Goals:**
- 사전정의 태그 체계(taxonomy) 도입 — 별도 변경으로 진행
- 태그 자동완성/검색 UI
- 사용자별 관심 태그 저장/구독 기능
- 피드 추천 알고리즘(`GET /api/feed`) 변경 — 이번 변경은 `GET /api/pins` 기반 필터링

## Decisions

### 1. 인기 태그 조회는 별도 엔드포인트로 분리

`GET /api/tags/popular?limit=30`

**근거:** 태그 목록은 핀 목록과 독립적으로 캐싱/로딩할 수 있다. SSR 시 `Promise.all`로 병렬 fetch 가능.

**대안:** 프론트엔드에서 하드코딩 — 데이터 기반이 아니라 유지보수 부담.

### 2. SQL: `unnest + GROUP BY` 집계

```sql
SELECT tag, COUNT(*) AS count
FROM (SELECT unnest(tags) AS tag FROM pins) AS t
GROUP BY tag ORDER BY count DESC LIMIT $1;
```

**근거:** 기존 GIN 인덱스와 관계없이 전체 스캔이지만, MVP 데이터 규모에서는 충분. 추후 materialized view나 Redis 캐시로 전환 가능.

**대안:** 별도 `tags` 테이블 관리 — 정규화 비용 대비 현 단계에서 과도함.

### 3. 태그 필터 UI는 FieldFilter 아래 별도 줄

`FieldFilter` → `TagFilter` 순서로 쌓는다. 두 필터는 AND 관계 (분야 + 태그 동시 적용).

**근거:** field 제거 피벗이 예정되어 있지만, 현재 코드에서는 field가 살아 있으므로 기존 UI를 유지하면서 태그 필터를 추가하는 게 안전하다. 피벗 시 FieldFilter만 제거하면 됨.

**대안:** FieldFilter 대체 — 피벗이 아직 적용되지 않아 시기상조.

### 4. 다중 태그 선택 (OR)

사용자가 여러 태그를 동시에 선택할 수 있으며, 선택된 태그 중 하나라도 포함된 핀이 반환된다 (PostgreSQL `&&` operator). URL: `?tags=태그1,태그2`.

**근거:** 단일 선택은 너무 제한적. OR 필터가 탐색성(discoverability)에 유리하고, 백엔드가 이미 overlap(`&&`)으로 동작함.

### 5. FeedContainer에서 태그 변경 감지

`searchParams.get("tags")`를 watch하여 field 변경과 동일한 패턴으로 리로드. offset 초기화 포함.

**근거:** 기존 field 필터 패턴(`reloadField`)을 그대로 확장하면 일관성 유지.

## Risks / Trade-offs

- **인기 태그 쿼리 성능**: 핀 수가 수만 건을 넘으면 `unnest` 전체 스캔이 느려질 수 있다 → SSR 캐시(revalidate) 또는 Redis TTL로 대응 가능
- **태그 칩 오버플로**: 인기 태그가 30개이면 좁은 화면에서 가로 스크롤 필요 → `overflow-x-auto`로 처리, FieldFilter와 동일 패턴
- **field + tags 조합 시 빈 결과**: 좁은 필터 조합에서 결과가 0건일 수 있음 → 기존 EmptyState 컴포넌트로 충분
