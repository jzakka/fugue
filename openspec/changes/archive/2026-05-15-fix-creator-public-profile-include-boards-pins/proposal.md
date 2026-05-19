## Why

`openspec/specs/profile/spec.md`의 Requirement `유저 프로필을 조회한다` Scenario "공개 프로필 조회"는 다음 SHALL을 묶고 있다.

- **WHEN** 유저 ID로 프로필을 조회하면
- **THEN** 닉네임, 아바타, 보드 목록, 핀 목록이 반환된다

THEN 절은 네 종류의 데이터(`닉네임`, `아바타`, `보드 목록`, `핀 목록`)가 프로필 조회 응답 페이로드에 함께 반환되어야 한다고 명시한다. 별도 endpoint들의 조합으로 분산해 채우는 형태는 spec text에 명시되어 있지 않다.

`apps/api/internal/creator/handler.go:37-64` `GetByID`(`GET /api/creators/{id}`)는 `h.q.GetCreator` + `h.q.CountPinsByCreator`만 조회하고 `toPublicDTO(creator, workCount)`를 직렬화한다. `apps/api/internal/creator/dto.go:9-15` `CreatorPublicDTO`는 `id`, `nickname`, `avatar_url`, `pin_count`, `created_at` 다섯 필드만 가진다. `boards`·`pins` 키는 응답 페이로드에 존재하지 않는다.

결과: 공개 프로필 조회 응답이 spec의 THEN 절에서 요구하는 `보드 목록, 핀 목록`을 제공하지 않으므로 Scenario "공개 프로필 조회" SHALL이 production에서 enforce되지 않는다.

## What Changes

- `GET /api/creators/{id}` 응답 페이로드에 `boards`·`pins` 두 키를 새로 포함한다. 두 키는 각각 공개 보드 요약 목록과 최근 핀 요약 목록이다.
- `boards`: 공개 보드만 최신 갱신순으로 일정 상한까지 포함한다. 비공개 보드는 호출자 인증 여부와 무관하게 본 응답에 노출하지 않는다(spec의 "공개 프로필" 범위).
- `pins`: 그 유저의 핀을 최신순으로 일정 상한까지 포함한다.
- 두 목록은 페이지네이션 메타데이터를 갖지 않으며, 더 많은 항목은 기존 endpoint(`GET /api/boards?creator_id=...`, `GET /api/pins?creator_id=...`)로 조회한다.
- 새 sqlc 쿼리 `ListPublicBoardsByCreatorLimited`를 추가해 공개 보드 목록에 SQL LIMIT를 부착한다. 핀 목록은 기존 `ListPinsByCreator`(LIMIT/OFFSET 지원)를 재사용한다.
- `CreatorPublicDTO`(`apps/api/internal/creator/dto.go`)를 확장해 `Boards []BoardSummary`·`Pins []PinSummary` 필드를 추가한다. 두 요약 타입은 creator 패키지 내부에 가볍게 정의해 외부 패키지의 무거운 응답 타입을 재사용하지 않는다(응답 안에서 creator 정보 중복 회피).
- `GetMe`(`GET /api/creators/me`)의 응답 형태는 본 change에서 변경하지 않는다. 본 change는 spec의 "공개 프로필 조회" Scenario만 닫는다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities

- `profile`: 기존 Requirement `유저 프로필을 조회한다` Scenario "공개 프로필 조회"의 SHALL 본문은 변경하지 않는다. ADDED Requirement `공개 프로필 조회 응답에 보드 요약과 핀 요약을 포함한다`(SHALL)를 추가해 응답 페이로드 형태를 spec 수준에서 enforce한다. 본 Requirement는 기존 Requirement의 의미를 좁히거나 넓히지 않으며, 기존 Scenario "존재하지 않는 유저"의 행위(404)도 변경하지 않는다.

## Impact

- 영향 코드: `apps/api/internal/creator/handler.go`(GetByID에서 boards/pins fetch), `apps/api/internal/creator/dto.go`(BoardSummary/PinSummary 신설, CreatorPublicDTO 확장), `apps/api/db/queries/boards.sql`(ListPublicBoardsByCreatorLimited 추가), sqlc 생성 산출물(`apps/api/internal/db/*.go`).
- 외부 컨트랙트 변경: `GET /api/creators/{id}` 응답에 `boards`·`pins` 두 키가 새로 추가된다. 기존 필드(`id`, `nickname`, `avatar_url`, `pin_count`, `created_at`)는 변경 없이 유지된다. 추가 키는 새 필드라서 기존 클라이언트가 무시해도 안전하다.
- DB 마이그레이션 없음. 인프라 변경 없음. 새 sqlc 쿼리는 기존 `boards` 테이블의 인덱스(`creator_id, is_public`)만 사용한다.
- 운영 지표: `GET /api/creators/{id}` 한 번당 `ListPublicBoardsByCreatorLimited`(보드 N=20 상한 단일 쿼리)와 `ListPinsByCreator`(핀 N=12 상한 단일 쿼리) 두 개의 추가 SELECT가 발생한다. 두 쿼리 모두 `creator_id` 인덱스를 사용하며 결과 크기가 상한으로 묶여 있다.
