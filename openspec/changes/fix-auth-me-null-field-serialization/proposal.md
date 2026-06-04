## Why

`apps/api/internal/auth/handler.go`의 `Me`(`GET /api/auth/me`, L223-224)가 `avatar_url`·`email`을 `sql.NullString`의 `.String` 필드로 `.Valid` 검사 없이 직접 직렬화한다. 컬럼이 NULL이면 Go zero value `""`가 직렬화되어 응답이 `{"avatar_url":"","email":""}`가 된다.

동일 인증 Creator를 반환하는 `GET /api/creators/me`(`creator.GetMe` → `toPrivateDTO`, `apps/api/internal/creator/dto.go:76`)는 같은 NULL 필드를 `null`로 반환한다. 즉 본 결함은 spec text 위반이라기보다 **같은 엔티티의 같은 필드를 두 JWT 프로필 엔드포인트가 다르게 직렬화하는 baseline 불일치**다.

baseline은 `toPrivateDTO`/`toPublicDTO`(creator), `pin/dto.go`의 7개 매퍼, `boards`/`feed` 핸들러 전반에 걸친 코드베이스 정전(canonical) 패턴 — nullable 컬럼을 `if x.Valid { p = &x.String }` 가드 후 `*T`로 변환해 NULL을 JSON `null`로 노출 — 이며, 본 change는 `auth.Me`를 그 baseline에 맞춘다. 빈 문자열은 "값이 설정되지 않음(null)"을 표현하지 못해 클라이언트가 두 endpoint에서 동일 필드의 부재를 다르게 해석하게 된다.

## What Changes

- `auth.Me`가 `avatar_url`·`email`을 코드베이스 정전 패턴(`if .Valid` 가드 → 미설정 시 JSON `null`)으로 직렬화한다. `id`·`nickname` 키와 외부 라우트는 변경하지 않는다.
- 값이 설정된 경우의 응답은 변하지 않는다(기존과 동일한 문자열 노출).

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities

- `auth`: 새 Requirement `인증된 유저의 프로필을 노출한다`를 추가해, `GET /api/auth/me`가 미설정 `avatar_url`·`email`을 JSON `null`로 직렬화함을 명시한다(빈 문자열 금지). 기존 Requirement들의 의미는 변경하지 않는다.

## Impact

- 영향 코드: `apps/api/internal/auth/handler.go`의 `Me` 함수 한 곳.
- 응답 계약: NULL `avatar_url`·`email` → 기존 `""`에서 `null`로 변경(`GET /api/creators/me`와 수렴). 값이 있는 경우와 다른 키(`id`·`nickname`)는 불변.
- 의존성, DB 스키마, 마이그레이션 없음. 롤백은 단일 함수 되돌리기로 가능.
