## Why

연관 핀 API(`GET /api/pins/{id}/related`)가 매 요청마다 SQL 에러를 발생시킨다. ORDER BY 절에서 `p.tags & $2::text[]` (배열 교집합)를 사용하는데, PostgreSQL은 text 배열에 `&` 연산자를 지원하지 않는다. 프론트엔드가 에러를 조용히 무시하므로 핀 상세 페이지 하단에 연관 작품이 전혀 표시되지 않는 상태다.

## What Changes

- 연관 핀 SQL 쿼리의 태그 일치도 정렬 로직을 PostgreSQL text 배열에서 동작하는 방식으로 수정
- sqlc 코드 재생성
- 프론트엔드 변경 없음 (API 응답 형식 동일)

## Capabilities

### New Capabilities

없음

### Modified Capabilities

없음 (기존 `feed` 스펙의 "연관 작품을 제공한다" 요구사항은 그대로 유효. 구현 버그 수정만 필요)

## Impact

- `apps/api/db/queries/pins.sql` — RelatedPins 쿼리 수정
- `apps/api/internal/db/pins.sql.go` — sqlc 재생성
- 프론트엔드, API 핸들러, 라우팅 변경 없음
