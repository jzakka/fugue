# 개인화 피드 페이지네이션의 페이지 간 작품 중복 차단

## Why

`openspec/specs/feed/spec.md` Requirement `개인화된 추천 피드를 제공한다` Scenario "피드 페이지네이션"은 다음 SHALL을 명시한다.

- **WHEN** 피드의 다음 페이지를 요청하면
- **THEN** 이전 페이지에 포함되지 않은 작품이 반환된다

그러나 `apps/api/internal/feed/handler.go`의 `buildPersonalizedFeed`(authenticated + `pinCount >= 10` 분기)는 페이지 offset을 underlying 쿼리에 전혀 전달하지 않는다. 결과적으로 1번째 페이지와 2번째 페이지가 거의 동일한 작품을 반환해 위 SHALL이 production에서 enforce되지 않는다.

근거 위치:
- `apps/api/internal/feed/handler.go:155-159`: `RecommendByTags` 호출이 OFFSET을 전달하지 않음
- `apps/api/internal/feed/handler.go:177-181`: `RecommendByMediaType` 호출이 OFFSET을 전달하지 않음
- `apps/api/internal/feed/handler.go:199-204`: `ListPinsWithCreator`(latest) 호출이 `Offset: 0` 하드코딩
- `apps/api/internal/feed/handler.go:223-228`: fill-gap 호출이 `Offset: int32(len(latestRows))` 사용 — 페이지 offset이 아니라 latest 길이(상수 10)
- `apps/api/internal/feed/handler.go:284-292`: `buildNextCursor`가 offset을 누적하지만 어떤 underlying 쿼리에도 그 offset이 전달되지 않음
- `apps/api/db/queries/interactions.sql:26-42`: `RecommendByTags` SQL이 OFFSET 인자를 받지 않음
- `apps/api/db/queries/interactions.sql:44-56`: `RecommendByMediaType` SQL이 OFFSET 인자를 받지 않음

cold-start/unauth 분기인 `buildLatestFeed`는 `Offset: int32(offset)`를 정상 전달하므로(handler.go:114-134) 영향받지 않는다. 본 결함은 개인화 분기에만 발생한다.

## What Changes

- 신규 Requirement `개인화 피드의 페이지네이션은 페이지 간 작품 중복을 반환하지 않는다`를 `feed` capability에 추가한다. 본 Requirement는 기존 Scenario "피드 페이지네이션"의 SHALL이 production에서 enforce되도록 보장하는 wiring 계약이다.
- `RecommendByTags`·`RecommendByMediaType` SQL 쿼리에 OFFSET 파라미터를 추가하고 sqlc를 재생성한다.
- `buildPersonalizedFeed`에서 페이지 offset을 세 underlying 쿼리(`RecommendByTags`, `RecommendByMediaType`, `ListPinsWithCreator` latest, fill-gap)에 일관되게 전파한다.
- 단위 테스트로 페이지 1·2 응답의 작품 ID 교집합이 비어 있음을 검증한다.

## Impact

- Affected specs: `feed` (신규 Requirement 1건)
- Affected code:
  - `apps/api/db/queries/interactions.sql` (2개 쿼리 시그니처 변경)
  - `apps/api/internal/db/interactions.sql.go` (sqlc 재생성)
  - `apps/api/internal/feed/handler.go` (`buildPersonalizedFeed` 본문)
  - `apps/api/internal/feed/handler_optional_auth_test.go`는 변경 안 함, 새 테스트 파일 추가
- 회귀 위험: `RecommendByTags`·`RecommendByMediaType` 호출자가 feed 패키지밖에 없는지 확인 후 시그니처 변경. 캐시 키 `feed:{userID}:{limit}:{offset}`는 이미 offset 포함이므로 캐시 충돌 없음.
- 마이그레이션: 스키마 변경 없음. sqlc 코드 재생성만 필요.
- 롤백: 본 PR을 revert하면 SQL·핸들러·생성 코드가 동시에 이전 상태로 돌아간다. 운영 데이터 영향 없음.
