## Context

메인 피드 페이지는 현재 미디어 타입 필터(`?media_type=image|audio|video`)만 제공한다. `FieldFilter` 컴포넌트가 URL 쿼리 파라미터로 동작하며, `FeedContainer`가 `fetchPins({ media_type })` 호출로 필터링한다.

핀-태그 관계는 `pin_tags` 조인 테이블로 관리되며, `tags` 테이블에 사전 정의된 태그(145개, 6개 카테고리)가 존재한다. `GET /api/tags`로 전체 태그 조회가 가능하고, `GET /api/pins?tag_ids=uuid1,uuid2`로 태그 기반 필터링이 이미 백엔드에서 지원된다. 빠져 있는 것은 (1) 인기 태그 집계 API와 (2) 사용자가 태그를 선택하는 UI다.

## Goals / Non-Goals

**Goals:**
- 메인 피드에서 인기 태그 기반 필터링 UI 제공
- `pin_tags` 테이블에서 인기 태그를 집계하는 API 엔드포인트 제공
- 태그 필터 상태를 URL 쿼리 파라미터로 관리 (공유/북마크 가능)
- 미디어 타입 필터와 태그 필터를 조합하여 사용 가능

**Non-Goals:**
- 태그 자동완성/검색 UI (핀 생성 폼에 이미 존재)
- 사용자별 관심 태그 저장/구독 기능
- 피드 추천 알고리즘(`GET /api/feed`) 변경 — 이번 변경은 `GET /api/pins` 기반 필터링

## Decisions

### 1. 인기 태그 조회는 별도 엔드포인트로 분리

`GET /api/tags/popular?limit=20` (기본값 20, 최대 50)

**근거:** 태그 전체 목록(`GET /api/tags`)은 145개로 피드 필터에는 과다. `pin_tags`에서 실제로 사용된 태그만 빈도순으로 제공해야 탐색성이 높아진다. SSR 시 `Promise.all`로 병렬 fetch 가능.

**대안:** 프론트엔드에서 전체 태그를 fetch한 후 정렬 — N+1 쿼리 없이 가능하나, 사용 빈도 정보가 없어 인기 순 정렬 불가.

### 2. SQL: `pin_tags GROUP BY` 집계

```sql
SELECT t.id, t.name, t.slug, t.category, COUNT(*) AS pin_count
FROM pin_tags pt
JOIN tags t ON t.id = pt.tag_id
GROUP BY t.id, t.name, t.slug, t.category
ORDER BY pin_count DESC
LIMIT $1;
```

**근거:** `pin_tags`에 `idx_pin_tags_tag` 인덱스가 있으므로 GROUP BY가 효율적. MVP 규모에서 충분.

### 3. 태그 필터 UI는 미디어 타입 필터 아래 별도 줄

`FieldFilter` (미디어 타입) → `TagFilter` (인기 태그) 순서로 쌓는다. 두 필터는 AND 관계.

### 4. 다중 태그 선택 (OR)

사용자가 여러 태그를 동시에 선택할 수 있으며, 선택된 태그 중 하나라도 연결된 핀이 반환된다. URL: `?tags=slug1,slug2` (slug 기반, 가독성 및 공유성).

**근거:** 백엔드 `GET /api/pins?tag_ids=`가 이미 `ANY()` 기반 OR 필터로 동작. 프론트엔드에서 slug→id 변환만 필요.

### 5. FeedContainer에서 태그 변경 감지

`searchParams.get("tags")`를 watch하여 media_type 변경과 동일한 패턴으로 리로드. offset 초기화 포함.

## Risks / Trade-offs

- **인기 태그 쿼리 성능**: 핀 수가 수만 건을 넘으면 GROUP BY 전체 스캔이 느려질 수 있다 → SSR revalidate 또는 Redis TTL로 대응 가능
- **태그 칩 오버플로**: 인기 태그가 30개이면 좁은 화면에서 가로 스크롤 필요 → `overflow-x-auto` 처리
- **slug→id 변환**: URL에 slug를 쓰되 API에는 id를 보내야 함 → 인기 태그 응답에 id+slug 모두 포함하여 프론트에서 매핑
- **인기 태그에 없는 slug**: 오래된 공유 링크에서 더 이상 인기가 아닌 태그 slug가 URL에 포함된 경우 → 해당 slug 무시 (graceful degradation)
