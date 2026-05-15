## Context

`openspec/specs/profile/spec.md` Requirement `유저 프로필을 조회한다` Scenario "공개 프로필 조회"는 응답 페이로드에 닉네임·아바타·보드 목록·핀 목록 네 가지가 함께 반환되어야 한다고 명시한다.

`apps/api/internal/creator/handler.go:37-64` `GetByID`는 그중 닉네임·아바타·`pin_count`(보드/핀 자체 목록이 아니라 핀 수)만 반환한다. `boards`·`pins` 키는 응답 페이로드에 존재하지 않는다.

기존 `apps/api/db/queries/boards.sql`의 `ListPublicBoardsByCreator`는 LIMIT가 없는 공개 보드 목록을 반환한다. 응답 크기 폭발을 방지하기 위해 본 change는 LIMIT 파라미터를 가진 새 쿼리를 도입한다. 핀 쪽은 기존 `ListPinsByCreator`가 이미 LIMIT/OFFSET을 지원하므로 재사용한다.

직전 사이클(`fix-boards-public-get-optional-jwt`, `fix-feed-route-optional-jwt`)의 패턴은 핸들러 본문을 건드리지 않는 wiring-only 변경이었다. 본 change는 핸들러 본문에 fetch + 응답 합성 로직이 추가되므로 변경 폭이 한 단계 더 크다. 그래서 추가 sqlc 쿼리·dto 타입·핸들러 단위 테스트를 동반한다.

## Goals / Non-Goals

**Goals:**
- `GET /api/creators/{id}` 응답에 `boards`(공개 보드 요약 배열)와 `pins`(핀 요약 배열) 두 키가 항상 포함된다(SHALL).
- `boards`는 공개 보드만, 최근 갱신순, 상한 N=20까지 반환한다.
- `pins`는 최신순, 상한 N=12까지 반환한다.
- 기존 응답 필드(`id`, `nickname`, `avatar_url`, `pin_count`, `created_at`)는 변경하지 않는다.
- 존재하지 않는 유저 ID에 대한 404 응답(기존 Scenario "존재하지 않는 유저")은 그대로 유지한다.
- 단위 테스트가 두 키의 포함, 상한 적용, 공개 보드만 필터링됨을 검증한다.

**Non-Goals:**
- `GetMe`(`GET /api/creators/me`) 응답 형태 변경. 본 change는 공개 프로필 조회 Scenario만 닫는다.
- 비공개 보드 노출 정책 도입. 호출자가 본인 자신을 조회하더라도 본 응답에는 공개 보드만 포함된다. 비공개 보드는 기존 `GET /api/boards?creator_id=<self>` 라우트(직전 change에서 wiring 머지 완료)로 조회한다.
- 페이지네이션 메타데이터(`has_more`, `next_offset` 등) 도입. 본 응답은 페이지가 아닌 "요약"이며, 더 많은 항목은 별도 endpoint로 조회한다.
- pin 패키지의 `PinResponse` 재사용. 응답 안의 핀 요약은 외부 creator 정보를 다시 담을 필요가 없다(이미 외곽 응답이 그 creator 자체이므로 중복 회피).
- 새 인덱스 도입. `boards(creator_id, is_public)` 패턴은 기존 인덱스로 충분하다(기존 `ListPublicBoardsByCreator`가 같은 WHERE 절을 쓰고 있음).

## Decisions

### Decision 1: 응답 페이로드에 `boards`·`pins` 두 키를 새로 추가한다 — 별도 endpoint 분리는 spec 위반으로 본다

**선택**: `CreatorPublicDTO`에 `Boards []BoardSummary`·`Pins []PinSummary` 두 필드를 추가하고, `GetByID`에서 동일 요청 안에 두 fetch를 수행해 응답을 합성한다.

**대안**:
- (a) `boards`·`pins`를 본 응답이 아닌 별도 endpoint(`GET /api/boards?creator_id=...`, `GET /api/pins?creator_id=...`)로만 유지하고 spec text를 그렇게 약화하기 → spec text는 명시적으로 "프로필을 조회하면 ... 보드 목록, 핀 목록이 반환된다"고 한 응답 안에 함께 반환됨을 표현한다. spec text를 약화하는 것은 자의적 해석이다.
- (b) spec text를 강화해 "보드 목록·핀 목록은 페이지네이션을 지원해야 한다"로 SHALL을 추가 → 본 change 범위 밖이며 별도 design 결정이 필요하다.

**근거**: spec text를 그대로 enforce하는 가장 작은 변경은 응답 페이로드에 두 키를 추가하는 것이다. 페이지네이션과 비공개 보드 가시성은 별도 SHALL이 spec text에 없으므로 본 change에서 도입하지 않는다.

### Decision 2: `boards`는 공개 보드만 포함한다 — 본인이 본인 조회해도 비공개 보드는 노출하지 않는다

**선택**: `ListPublicBoardsByCreatorLimited`(`is_public = true` 필터 포함)를 사용한다. `GetByID`는 비인증 호출이 가능한 라우트이며, 인증 컨텍스트에 따라 분기하지 않는다.

**대안**:
- (a) 본인이 본인 조회 시 비공개 보드도 포함 → `GetByID`는 현재 인증 미들웨어 없이 등록되어 있다(main.go:153). `OptionalJWTMiddleware`를 부착해 owner 분기를 도입할 수 있으나, 본 change 범위가 (라우트 wiring 변경 + 응답 합성 + 새 SHALL)로 두 점이 커진다. 또한 본인 비공개 보드는 이미 `GET /api/boards?creator_id=<self>` 라우트가 직전 change에서 wiring 완료되어 클라이언트가 별도 조회 가능하다.
- (b) 본인이 본인 조회 시 `boards` 키를 아예 비우기 → 응답 일관성이 떨어진다.

**근거**: spec text는 "공개 프로필"이라고 명시한다. 본인이 본인 조회 시의 비공개 보드 노출은 별도 SHALL이 spec에 명시되지 않으므로 본 change에서 도입하지 않는다. 본인 비공개 보드 접근 경로는 기존 board 라우트로 이미 제공된다.

### Decision 3: `boards`·`pins` 상한은 각각 N=20·N=12 — 코드 상수로 둔다

**선택**: `apps/api/internal/creator/handler.go`에 패키지 상수 `maxPublicProfileBoards = 20`·`maxPublicProfileRecentPins = 12`를 두고 `ListPublicBoardsByCreatorLimited`와 `ListPinsByCreator` 호출에 LIMIT로 전달한다. spec text에는 정확한 N을 명시하지 않고 "일정 상한"으로 표현한다.

**대안**:
- (a) spec text에 N=20·N=12를 명시 → 상한이 향후 운영 데이터에 따라 조정될 가능성이 있다. spec change를 매번 동반하는 비용이 늘어난다.
- (b) 상한 없이 전체 목록 반환 → 응답 크기 폭발 위험. 한 유저가 핀 1000개 보유 시 응답 본문이 수십 MB로 늘어난다.

**근거**: 상한은 spec text가 명시한 SHALL의 일부가 아니라 시스템 운영 디테일이다. 다만 응답 크기 제한은 필수이므로 코드 상수로 강제하고, spec text는 "상한 있는 요약"이라고 표현한다. 추후 N 조정은 코드 변경만으로 가능하고 spec change를 동반하지 않는다.

### Decision 4: pin 요약은 creator 패키지 내부에 가벼운 `PinSummary` 타입으로 둔다 — `pin.PinResponse`를 재사용하지 않는다

**선택**: `apps/api/internal/creator/dto.go`에 `PinSummary`(필드: `id`, `media_url`, `media_type`, `title`, `og_image`, `created_at`)와 `BoardSummary`(필드: `id`, `name`, `description`, `is_public`, `updated_at`)를 정의한다.

**대안**:
- (a) `pin.PinResponse`(creator 필드 포함, tags 필드 포함) 재사용 → 응답 안에서 creator 정보가 두 번 들어간다(외곽 응답 자체가 그 creator이므로). 클라이언트가 무시해도 무해하지만 응답 본문이 불필요하게 커진다.
- (b) `boards` 패키지에서 BoardSummary를 만들고 export → boards 패키지를 가볍게 두기 위해 본 change는 creator 패키지 내부에 둔다. 차후 다른 호출자가 같은 요약을 필요로 하면 boards/pin 패키지로 옮길 수 있다.

**근거**: 본 change의 응답 본문 안에서는 외곽이 이미 creator 식별 정보이므로 핀 요약 안에 creator 정보를 중복해 담을 필요가 없다. 작은 요약 타입을 두면 응답 크기가 작아지고 dto.go 변환 함수가 단순해진다.

### Decision 5: 새 sqlc 쿼리 `ListPublicBoardsByCreatorLimited`를 추가한다 — 기존 `ListPublicBoardsByCreator`는 변경하지 않는다

**선택**: `apps/api/db/queries/boards.sql`에 다음 쿼리를 추가한다.

```sql
-- name: ListPublicBoardsByCreatorLimited :many
SELECT * FROM boards
WHERE creator_id = $1 AND is_public = true
ORDER BY updated_at DESC
LIMIT $2;
```

**대안**:
- (a) 기존 `ListPublicBoardsByCreator`에 LIMIT 파라미터를 추가 → 기존 호출자(`apps/api/internal/boards/handler.go:ListByCreator`)가 LIMIT 없는 전체 목록을 기대한다. 시그니처 변경이 호출 패턴 변경을 강제한다.
- (b) 코드에서 슬라이스 자르기(`rows[:20]`) → SQL이 전체 보드를 메모리로 가져온 뒤 잘라야 한다. 보드 보유 수가 큰 유저에서 비효율이다.

**근거**: SQL 단에서 LIMIT을 적용하는 것이 자연스럽고, 기존 쿼리를 손대지 않으면 기존 호출자의 회귀가 0이다. 두 쿼리는 같은 인덱스(`boards.creator_id`)를 사용하므로 인프라 변경 없이 추가된다.

### Decision 6: 핀 쿼리는 기존 `ListPinsByCreator`를 재사용한다

**선택**: `ListPinsByCreator(creator_id, mediaType='', tagIDs=NULL, limit=12, offset=0)`로 호출한다.

**대안**:
- 별도 `ListLatestPinsByCreator` 쿼리 신설 → 기존 쿼리가 이미 동일한 정렬·필터·LIMIT 의미를 지원한다. 중복 쿼리는 유지 비용만 늘린다.

**근거**: 기존 쿼리의 시그니처가 본 용도에 그대로 들어맞는다. mediaType/tagIDs를 비워서 전체 핀을 최신순으로 N개 가져오는 패턴은 이미 `apps/api/internal/pin/handler.go:listByCreator`가 사용한다.

## Risks / Trade-offs

- **[Risk] 응답 본문 크기 증가**: 기존 `GET /api/creators/{id}` 응답은 1KB 미만의 작은 페이로드였다. 본 change 후 최악의 경우 약 12개 핀(media_url URL, title, og_image 포함) + 20개 보드(name, description 포함)가 추가되어 응답 본문이 ~5–10KB로 증가할 수 있다. **Mitigation**: 상한 N=12·N=20과 가벼운 요약 타입(creator/tags 중복 제외)으로 응답 본문을 묶었다. 추가 줄이려면 후속 change에서 `description` 트렁케이션을 도입할 수 있다.
- **[Risk] DB 쿼리 수 증가**: `GetByID` 호출당 SELECT 3개(creator + count + boards) + 1개(pins) = 4개. 기존은 2개. **Mitigation**: 두 추가 쿼리 모두 `creator_id` 인덱스 + LIMIT로 결과가 작다. 운영 환경에서 RPS가 매우 높아지면 application-level 캐시를 별도 change에서 추가할 수 있다.
- **[Trade-off] 본인이 본인 조회 시 비공개 보드 미노출**: 본인이 본인 프로필을 조회해도 비공개 보드는 응답에 포함되지 않는다. 본인은 별도 라우트(`GET /api/boards?creator_id=<self>`)로 비공개 보드를 본다. 본 응답은 "공개 프로필"의 보드 목록임을 명시한다.
- **[Trade-off] 외부 컨트랙트에 키 두 개 추가**: 기존 클라이언트가 두 새 키를 무시하면 그대로 동작한다. 다만 fugue web 클라이언트가 본 두 키를 활용하려면 별도 디자인 트랙 작업이 필요하다(본 시스템 트랙 범위 밖).
