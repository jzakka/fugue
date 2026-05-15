# Tasks

## 1. SQL 시그니처 확장

- [ ] 1.1 `apps/api/db/queries/interactions.sql`의 `RecommendByTags` 쿼리에 OFFSET 파라미터 추가.
- [ ] 1.2 같은 파일의 `RecommendByMediaType` 쿼리에 OFFSET 파라미터 추가.
- [ ] 1.3 `cd apps/api && sqlc generate`로 `apps/api/internal/db/interactions.sql.go` 재생성.

## 2. 핸들러 wiring

- [ ] 2.1 `apps/api/internal/feed/handler.go` `buildPersonalizedFeed` 시그니처에 `offset int` 인자 추가.
- [ ] 2.2 `RecommendByTags`·`RecommendByMediaType` 호출에 page offset 전달.
- [ ] 2.3 latest용 `ListPinsWithCreator` 호출에 page offset 전달.
- [ ] 2.4 fill-gap용 `ListPinsWithCreator` 호출 offset을 `int32(offset + len(latestRows))`로 변경.
- [ ] 2.5 `GetFeed`에서 `buildPersonalizedFeed`에 offset 전달.

## 3. OpenSpec 스펙 갱신

- [ ] 3.1 `specs/feed/spec.md` 델타에 신규 Requirement `개인화 피드의 페이지네이션은 페이지 간 작품 중복을 반환하지 않는다`를 ADDED로 정의(scenarios ≥ 3).

## 4. 테스트

- [ ] 4.1 feed 패키지 mockQuerier를 신규 메서드 시그니처(`RecommendByTags`·`RecommendByMediaType`·`ListPinsWithCreator`)에 맞추어 갱신.
- [ ] 4.2 페이지 1·2를 연속 호출했을 때 응답 `pins[].id` 교집합이 비어 있음을 검증하는 테스트 추가.
- [ ] 4.3 offset이 세 underlying 쿼리에 모두 전파됨을 mockQuerier 호출 파라미터로 검증.

## 5. 검증

- [ ] 5.1 `cd apps/api && go build ./...` 통과.
- [ ] 5.2 `cd apps/api && go test ./internal/feed/... ./internal/db/...` 통과.
- [ ] 5.3 `openspec validate fix-feed-personalized-pagination-no-cross-page-duplicates --strict` 통과.
