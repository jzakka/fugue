## Context

Fugue는 크로스미디어 큐레이션 플랫폼. 현재 NavBar에 검색 input이 있지만 `disabled` 상태. 백엔드에 검색 API가 전혀 없고, 기존 핀 목록은 field/tags 필터만 지원한다.

최근 핀 모델 피벗(2026-04-06)으로 분야(field) 필드가 제거 예정이고, 태그는 사전정의 방식으로 전환됐다. 검색 구현 시 field 컬럼은 무시한다.

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
- field 컬럼 제거 migration (별도 PR)

## Decisions

### D1: 검색 엔진 — pg_trgm (trigram)

PostgreSQL 내장 extension. GIN 인덱스로 similarity() 기반 유사도 검색.

- **대안 A**: ILIKE — 단순하지만 인덱스 활용 불가, 유사도 정렬 불가
- **대안 B**: tsvector/tsquery — 한글 형태소 분석기(mecab) 설정이 복잡
- **대안 C**: pg_bigm — CJK 최적화지만 별도 extension 설치 필요, 호스팅 호환성 문제

**선택 이유**: pg_trgm은 Docker/클라우드 어디서나 기본 제공. 한글 3글자 이상에서 잘 동작. similarity() 함수로 랭킹 정렬 가능.

### D2: 한글 2자 이하 처리 — ILIKE fallback

pg_trgm은 3글자(trigram) 단위로 쪼개므로 2자 이하 검색어에서 매칭 품질이 낮다.

- `len([]rune(q)) > 2` (공백 제외, 유니코드 rune 단위) → pg_trgm similarity()
- `len([]rune(q)) <= 2` → `ILIKE '%q%'` fallback
- 두 방식은 분기(합치지 않음)

### D3: API 구조 — 통합 엔드포인트

```
GET /api/search?q=&type=all|pins|creators|boards&tags=tag1,tag2&limit=&offset=
```

- `type=all` 응답: `{ "pins": [...], "creators": [...], "boards": [...], "top_tags": [...] }`
- `type=pins|creators|boards` 응답: 해당 카테고리 + `has_more` + `top_tags`
- 자동완성 = `type=all&limit=5` 호출로 재활용 (top_tags 무시)
- `tags` 파라미터: comma-separated, 최대 5개, 사전정의 태그와 정확 매칭 (AND)

**대안**: 카테고리별 분리 엔드포인트 — 네트워크 3회 호출 필요, 불필요한 복잡성.

### D4: similarity 임계값 — 0.1

`SET pg_trgm.similarity_threshold = 0.1` 또는 `WHERE similarity(col, $1) > 0.1`. 낮게 설정하여 넓은 결과 반환, `ORDER BY similarity DESC`로 관련성 정렬.

### D5: 태그 검색 — 통합 포함 + 정확 매칭 부스트

검색어가 제목에도 매칭되고 태그에도 매칭될 때, 태그 정확 매칭된 핀이 더 높은 점수를 받도록 랭킹에 가산점 부여.

```sql
ORDER BY
  similarity(p.title, $1) +
  CASE WHEN $1 = ANY(p.tags) THEN 0.5 ELSE 0 END
  DESC
```

### D6: 보드 검색 보안 — is_public 필터

보드 검색 쿼리에 `WHERE is_public = true` 조건 필수. 비공개 보드 이름이 검색 결과에 노출되면 안 된다.

### D7: 태그 필터 칩 — 상위 100건 집계

검색 결과 상위 100건에서 `unnest(tags)` + `GROUP BY` + `COUNT DESC LIMIT 10`으로 태그 집계. API 응답에 `top_tags` 필드로 반환.

### D8: 자동완성 드롭다운 렌더링

섹션 헤더로 구분: 핀 최대 3개, 크리에이터 최대 1개, 보드 최대 1개. Enter 키 → `/search?q=` 전체 결과 페이지로 이동. debounce 300ms.

### D9: 최근 검색어 — localStorage

프론트엔드 전용. `fugue_recent_searches` 키. 최대 5개, FIFO. 각 항목에 삭제(X) 버튼. 검색바 focus 시 드롭다운에 표시.

## Risks / Trade-offs

- **[pg_trgm 한글 2자]** → ILIKE fallback으로 대응. 1자 검색은 결과가 너무 넓을 수 있으나 limit으로 제어.
- **[GIN 인덱스 크기]** → 현재 데이터 규모에서 무시 가능. 수십만 건 이상이면 인덱스 크기 모니터링 필요.
- **[통합 엔드포인트 복잡도]** → type 파라미터로 3개 카테고리 + top_tags를 한 핸들러에서 처리. 핸들러가 비대해질 수 있으나, 검색은 단일 도메인이므로 분리보다 통합이 적절.
- **[similarity 0.1 낮은 임계값]** → 관련 없는 결과가 포함될 수 있으나 랭킹 정렬로 상위에 관련 결과 노출. 향후 데이터 기반 튜닝.

## Migration Plan

1. DB migration: `CREATE EXTENSION IF NOT EXISTS pg_trgm` + GIN 인덱스 3개 (pins.title, creators.nickname, boards.name)
2. 백엔드: search.sql 쿼리 → sqlc generate → search handler → router 등록
3. 프론트엔드: NavBar 활성화 + 검색 결과 페이지 + 자동완성 컴포넌트
4. Rollback: migration down으로 인덱스 + extension 제거, 라우터에서 엔드포인트 제거

## Open Questions

없음. CEO 리뷰에서 모든 기술 결정 완료.
