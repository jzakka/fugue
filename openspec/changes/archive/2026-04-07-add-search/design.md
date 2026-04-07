## Context

Fugue는 크로스미디어 큐레이션 플랫폼. NavBar에 검색 input이 있지만 `disabled` 상태. 백엔드에 검색 API가 없고, 기존 핀 목록은 media_type/tag_ids 필터만 지원한다.

핀-태그 관계는 `pin_tags` 조인 테이블로 관리되며, `tags` 테이블에 사전 정의된 태그가 존재한다. 분야(field) 필드는 제거 완료.

기술 스택: Go + Chi + sqlc (백엔드), Next.js App Router (프론트엔드), PostgreSQL + Redis.

## Goals / Non-Goals

**Goals:**
- 핀 제목+태그, 크리에이터 닉네임, 보드 이름을 통합 검색하는 API 제공
- pg_trgm 기반 유사도 검색 + 랭킹 정렬
- 자동완성 (as-you-type), 태그 필터 칩, 최근 검색어, Deep Link
- 비공개 보드가 검색 결과에 노출되지 않도록 보안 보장

**Non-Goals:**
- 시맨틱 검색, ML 기반 검색 랭킹
- 검색 분석/트렌딩 검색어
- 별도 검색 엔진(Elasticsearch, Meilisearch) 도입

## Decisions

### D1: 검색 엔진 — pg_trgm (trigram)

PostgreSQL 내장 extension. GIN 인덱스로 similarity() 기반 유사도 검색.

- **대안 A**: ILIKE — 단순하지만 유사도 정렬 불가
- **대안 B**: tsvector/tsquery — 한글 형태소 분석기(mecab) 설정이 복잡
- **대안 C**: pg_bigm — CJK 최적화지만 별도 extension, 호스팅 호환성 문제

**선택 이유**: pg_trgm은 Docker/클라우드 기본 제공. 한글 3글자 이상에서 잘 동작. similarity() 함수로 랭킹 정렬 가능.

### D2: 한글 2자 이하 처리 — ILIKE fallback

pg_trgm은 3글자(trigram) 단위이므로 2자 이하 검색어에서 매칭 품질이 낮다.

- `len([]rune(q)) > 2` → pg_trgm similarity()
- `len([]rune(q)) <= 2` → `ILIKE '%q%'` fallback
- 두 방식은 분기(합치지 않음)

### D3: API 구조 — 통합 엔드포인트

```
GET /api/search?q=&type=all|pins|creators|boards&tag_ids=uuid1,uuid2&limit=&offset=
```

- `type=all`: 핀/크리에이터/보드를 각각 limit개까지 반환 + top_tags
- `type=pins|creators|boards`: 해당 카테고리 + has_more + top_tags
- 자동완성 = `type=all&limit=5` 호출로 재활용
- `tag_ids` 파라미터: comma-separated UUID, 최대 5개, AND 필터 (핀 검색에만 적용)

### D4: similarity 임계값 — 0.1

`WHERE similarity(col, $1) > 0.1`. 낮게 설정하여 넓은 결과 반환, `ORDER BY similarity DESC`로 관련성 정렬.

### D5: 태그 검색 — pin_tags JOIN + 태그명 매칭 부스트

핀 검색 시 `pin_tags` 테이블을 JOIN하여 태그명이 검색어와 일치하는 핀에 가산점 부여.

```sql
ORDER BY
  similarity(p.title, $1) +
  CASE WHEN EXISTS (
    SELECT 1 FROM pin_tags pt JOIN tags t ON t.id = pt.tag_id
    WHERE pt.pin_id = p.id AND t.name = $1
  ) THEN 0.5 ELSE 0 END
  DESC
```

정확히 일치하는 태그명만 부스트하며, 부분 일치는 부스트하지 않는다. 사전 정의 태그는 짧고 명확한 이름이므로 완전 일치가 적절.

### D6: 보드 검색 보안 — is_public 필터

보드 검색 쿼리에 `WHERE is_public = true` 조건 필수.

### D7: 태그 필터 칩 — 검색 결과 상위 100건에서 집계

검색 결과 상위 100건의 핀에 연결된 태그를 `pin_tags` JOIN으로 집계. top 10개를 `top_tags` 필드로 반환.

### D8: 자동완성 드롭다운

섹션 헤더로 구분: 핀 최대 3개, 크리에이터 최대 1개, 보드 최대 1개. Enter 키 → `/search?q=` 전체 결과 페이지로 이동. debounce 300ms. 자동완성은 **3자 이상**에서만 발동. 2자 이하 검색은 Enter로 전체 검색 페이지에서만 가능.

### D9: NavBar 검색바 — Client Component 분리

현재 `NavBar.tsx`는 Server Component(`async function`)이므로 훅/이벤트 핸들러 사용 불가. 검색바 영역만 별도 Client Component(`SearchBar.tsx`)로 분리한다. NavBar 자체는 Server Component를 유지하여 `getAuthUser()` 서버 호출을 보존.

### D10: 최근 검색어 — localStorage

프론트엔드 전용. `fugue_recent_searches` 키. 최대 5개, FIFO. 각 항목에 삭제(X) 버튼. 검색바 focus 시 드롭다운에 표시.

## Risks / Trade-offs

- **pg_trgm 한글 2자**: ILIKE fallback으로 대응. 1자 검색은 결과가 넓을 수 있으나 limit으로 제어.
- **GIN 인덱스 크기**: 현재 데이터 규모에서 무시 가능. 수십만 건 이상이면 모니터링 필요.
- **통합 엔드포인트 복잡도**: type 파라미터로 3개 카테고리를 한 핸들러에서 처리. 핸들러 비대 가능하나, 검색은 단일 도메인이므로 통합이 적절.
- **similarity 0.1 낮은 임계값**: 관련 없는 결과 포함 가능하나 랭킹 정렬로 상위에 관련 결과 노출.
