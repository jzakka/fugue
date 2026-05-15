## 1. sqlc 쿼리 추가

- [ ] 1.1 `apps/api/db/queries/boards.sql`에 신규 쿼리 `ListPublicBoardsByCreatorLimited`를 추가한다. SELECT는 기존 `ListPublicBoardsByCreator`와 동일(`SELECT * FROM boards WHERE creator_id = $1 AND is_public = true ORDER BY updated_at DESC`)하고 `LIMIT $2`만 부착한다.
- [ ] 1.2 `cd apps/api && sqlc generate`로 `apps/api/internal/db/boards.sql.go`·`apps/api/internal/db/querier.go`를 갱신한다.

## 2. creator dto 확장

- [ ] 2.1 `apps/api/internal/creator/dto.go`에 다음 두 요약 타입을 신규 정의한다.
  - `BoardSummary{ID string, Name string, Description *string, IsPublic bool, UpdatedAt time.Time}` (JSON 키: `id`, `name`, `description`, `is_public`, `updated_at`)
  - `PinSummary{ID string, MediaURL string, MediaType string, Title string, OgImage *string, CreatedAt time.Time}` (JSON 키: `id`, `media_url`, `media_type`, `title`, `og_image`, `created_at`)
- [ ] 2.2 `CreatorPublicDTO`에 두 필드를 추가한다: `Boards []BoardSummary \`json:"boards"\``, `Pins []PinSummary \`json:"pins"\``.
- [ ] 2.3 `toPublicDTO` 시그니처를 `toPublicDTO(c db.Creator, pinCount int64, boards []BoardSummary, pins []PinSummary) CreatorPublicDTO`로 확장한다. nil 슬라이스는 빈 슬라이스로 정규화해 응답 JSON에 `null` 대신 `[]`가 직렬화되도록 보장한다.
- [ ] 2.4 두 요약 타입에 대해 sqlc row → 요약 타입 변환 함수를 dto.go에 추가한다(예: `toBoardSummary(b db.Board) BoardSummary`, `toPinSummary(row db.ListPinsByCreatorRow) PinSummary`).

## 3. creator 핸들러 fetch 추가

- [ ] 3.1 `apps/api/internal/creator/handler.go`의 `CreatorQuerier` 인터페이스에 다음 두 메서드를 추가한다.
  - `ListPublicBoardsByCreatorLimited(ctx, db.ListPublicBoardsByCreatorLimitedParams) ([]db.Board, error)`
  - `ListPinsByCreator(ctx, db.ListPinsByCreatorParams) ([]db.ListPinsByCreatorRow, error)`
- [ ] 3.2 패키지 상수 `maxPublicProfileBoards = 20`·`maxPublicProfileRecentPins = 12`를 정의한다.
- [ ] 3.3 `GetByID`에서 creator 조회 직후 두 fetch를 추가한다.
  - `boardRows, err := h.q.ListPublicBoardsByCreatorLimited(ctx, {CreatorID: id, Limit: maxPublicProfileBoards})`
  - `pinRows, err := h.q.ListPinsByCreator(ctx, {CreatorID: id, Column2: "", Column3: nil, Limit: maxPublicProfileRecentPins, Offset: 0})`
  - 두 fetch 중 어느 하나가 DB 에러를 반환하면 500을 반환한다(기존 핸들러 에러 처리 패턴 그대로).
- [ ] 3.4 fetched rows를 `toBoardSummary`/`toPinSummary`로 변환해 `toPublicDTO`에 전달한다.
- [ ] 3.5 `GetMe`·`UpdateMe`는 변경하지 않는다(`toPrivateDTO`는 본 change 범위 밖).

## 4. 단위 테스트

- [ ] 4.1 `apps/api/internal/creator/handler_public_profile_test.go`(신규)를 만든다. fake `CreatorQuerier`를 정의해 `GetCreator`·`CountPinsByCreator`·`ListPublicBoardsByCreatorLimited`·`ListPinsByCreator` 네 메서드의 반환값을 테스트별로 조정 가능하게 한다.
- [ ] 4.2 시나리오 (a): 보드 3개·핀 5개를 반환하는 fake에 대해 `GET /api/creators/{valid-id}`를 호출하고, 응답 JSON에 `boards`(길이 3)와 `pins`(길이 5) 두 키가 spec이 정한 필드로 포함되는지 검증한다.
- [ ] 4.3 시나리오 (b): 보드 0개·핀 0개를 반환하는 fake에 대해 두 키가 `null`이 아닌 빈 배열 `[]`로 직렬화되는지 검증한다.
- [ ] 4.4 시나리오 (c): fake에 `LIMIT` 인자가 각각 `maxPublicProfileBoards`·`maxPublicProfileRecentPins`로 전달되는지 검증한다(상한이 코드에 박혀 있음을 회귀 방지).
- [ ] 4.5 시나리오 (d): `GetCreator`가 `sql.ErrNoRows`를 반환하면 응답이 404이고 boards/pins fetch가 호출되지 않는지 검증한다(존재하지 않는 유저 Scenario 회귀 방지).
- [ ] 4.6 시나리오 (e): `ListPublicBoardsByCreatorLimited`가 에러를 반환하면 500을 반환하는지 검증한다.

## 5. 검증

- [ ] 5.1 `cd apps/api && go build ./...` 통과.
- [ ] 5.2 `cd apps/api && go test ./internal/creator/... ./internal/db/...` 통과.
- [ ] 5.3 `openspec validate fix-creator-public-profile-include-boards-pins --strict` 통과.
