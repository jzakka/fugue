# fix-search-tag-match-parity — Tasks

## 1. 쿼리 수정

- [x] 1.1 `apps/api/db/queries/search.sql`의 similarity 경로 태그 술어 4곳(`SearchPinsBySimilarity`의 CASE·WHERE EXISTS, `SearchPinsWithTagFilter`의 CASE·WHERE EXISTS)을 `t.name = $1`에서 `t.name ILIKE '%' || $1 || '%'`로 교체
- [x] 1.2 sqlc 재생성(`sqlc generate`)으로 `apps/api/internal/db/search.sql.go` 갱신, 파라미터 타입 변화 확인

## 2. 핸들러 반영

- [x] 2.1 `apps/api/internal/search/handler.go`의 `searchPins` similarity 분기 2곳에서 재생성된 파라미터 구조에 맞게 인자 구성 수정 (분기 구조는 유지)
- [x] 2.2 `db.SearchPinsBySimilarityParams`/`SearchPinsWithTagFilterParams`에 의존하는 기존 mock querier 테스트(`apps/api/internal/search/` 내 테스트 파일들)의 영향 확인 및 필요 시 갱신
- [x] 2.3 `apps/api` 디렉토리에서 `go build ./...`로 컴파일 확인

## 3. 테스트

- [x] 3.1 similarity 경로 태그 술어가 ILIKE 의미(대소문자 무시 부분 일치)를 갖는지 고정하는 테스트 추가 (생성 SQL 계약 테스트 또는 기존 관례 패턴)
- [x] 3.2 두 경로(2자 이하 / 3자 이상)의 태그 매칭 의미 일치를 검증하는 테스트 추가
- [x] 3.3 `apps/api` 디렉토리에서 `go test ./...` 통과 확인 (3.1 계약 테스트가 배치된 패키지 포함)

## 4. 검증

- [x] 4.1 docker-compose Postgres 기동이 가능하면 실제 DB에 시드 태그(`Art`, `illustration`)로 `q=art`·`q=illust` 검색이 태그 매칭되는지 QA 확인 (불가 시 사유 명시)
