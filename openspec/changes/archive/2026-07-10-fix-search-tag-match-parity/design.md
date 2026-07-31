# fix-search-tag-match-parity — Design

## Context

`GET /api/search`의 핀 검색은 검색어 길이에 따라 두 SQL 경로로 나뉜다 (`apps/api/internal/search/handler.go:171`의 `useSimilarity = RuneCount(q) > 2`).

- 2자 이하 → `SearchPinsByILIKE` / `SearchPinsILIKEWithTagFilter`: 태그 조건이 `t.name ILIKE '%' || $1 || '%'` (대소문자 무시 부분 일치)
- 3자 이상 → `SearchPinsBySimilarity` / `SearchPinsWithTagFilter`: 태그 조건이 `t.name = $1` (대소문자 구분 완전 일치), WHERE 포함 판정과 `+0.5` 가산점 CASE 두 곳 모두

같은 태그(`Art`)가 `ar`로는 검색되는데 `art`로는 검색되지 않는 비일관이 발생한다. 쿼리는 sqlc로 생성되며 (`apps/api/db/queries/search.sql` → `apps/api/internal/db/search.sql.go`), 핸들러 Go 코드는 파라미터를 그대로 넘길 뿐이다.

## Goals / Non-Goals

**Goals:**
- similarity 경로(무필터·태그필터 2개 쿼리)의 태그 매칭(WHERE EXISTS + 가산점 CASE, 총 4곳)을 ILIKE 경로와 동일한 `ILIKE '%' || $1 || '%'` 의미로 통일
- 두 경로의 태그 매칭 의미 일치를 고정하는 테스트 추가

**Non-Goals:**
- 제목(similarity) 매칭 로직·임계값 변경
- 태그 정규화(슬러그 기반 매칭 등) 도입 — 별도 개선 과제
- creators/boards 검색 경로 변경 (태그 개념 없음)
- 검색 API 계약(요청/응답 스키마) 변경

## Decisions

1. **`t.name = $1` → `t.name ILIKE '%' || $1 || '%'` 로 교체 (4곳: search.sql L11, L18, L50, L57)**
   - 대안 A: `LOWER(t.name) = LOWER($1)` (case-insensitive exact). 대소문자 문제는 풀리지만 부분 일치 후퇴(`illust` → `illustration` 불일치)는 남아 두 경로 의미가 여전히 어긋난다. 기각.
   - 대안 B: similarity 경로를 태그에도 `similarity()` 적용. pg_trgm 인덱스 부재 시 비용 증가 + ILIKE 경로와 의미가 또 달라짐. 기각.
   - 채택안은 기존 ILIKE 경로와 문자 그대로 동일한 술어라 의미 일치가 자명하다. sqlc 생성 파라미터 타입은 `$1`이 `similarity(p.title, $1)`과 공유되는 탓에 재생성 결과에 따라 달라질 수 있으며, 재생성 후 확인하여 핸들러를 맞춘다(Decision 3).

2. **WHERE 포함 판정과 가산점 CASE를 함께 교체**
   - 포함 판정만 바꾸면 태그로 걸린 핀이 가산점을 못 받아 순위가 비직관적으로 낮아진다(스펙의 "가산점 매칭 규칙 일관성" 요구 위반). 두 곳을 항상 같은 술어로 유지한다.

3. **sqlc 재생성으로 파라미터 타입 변화 흡수**
   - `= $1`은 `Similarity string` 단일 파라미터로 공유되었으나, ILIKE 연결식으로 바꾸면 sqlc가 별도 nullable 파라미터를 생성할 수 있다. 생성 결과에 맞춰 핸들러의 파라미터 구성만 최소 수정한다. 핸들러 분기 구조는 유지.

4. **테스트는 쿼리 텍스트 계약 + 핸들러 경로 테스트로 고정**
   - 실제 Postgres 없이도 회귀를 막기 위해, 생성된 SQL 상수에 similarity 경로 태그 술어가 ILIKE 의미를 갖는지 검증하는 테스트를 둔다(기존 프로젝트에 생성-SQL 계약 테스트 관례가 있는지 확인 후 동일 패턴 사용). docker-compose로 Postgres 기동이 가능하면 통합 검증(QA)로 실제 동작을 확인한다.

## Risks / Trade-offs

- [부분 일치로 확대되어 3자 이상 검색의 태그 매칭 결과가 늘어남] → 의도된 동작 변화다. 짧은 검색어 경로와 동일한 의미이므로 사용자 관점에서는 일관성 회복이다. 가산점도 부분 일치에 부여되어 순위가 다소 넓게 퍼질 수 있으나, 포함 판정과 동일 규칙 유지가 스펙 요구다.
- [`%`·`_` 등 LIKE 메타문자가 검색어에 포함되면 와일드카드로 해석됨] → 기존 ILIKE 경로(2자 이하)와 동일한 기존 동작이며 본 change의 범위 밖. 두 경로가 같은 동작을 공유하게 되므로 비일관은 아니다.
- [태그 부분 일치 EXISTS 서브쿼리의 인덱스 활용도 저하] → 태그 테이블은 소규모(핀당 최대 수 개)이고 EXISTS는 pin_id 조건으로 먼저 좁혀지므로 실질 비용 증가는 미미하다.

## Migration Plan

- DB 스키마 변경 없음. 쿼리 텍스트 변경 + sqlc 재생성 + 핸들러 파라미터 수정만 배포하면 된다. 롤백은 커밋 revert로 충분하다.

## Open Questions

- 없음
