## 1. 구현

- [x] 1.1 `apps/api/internal/auth/handler.go`의 `Me`에서 `avatar_url`·`email`을 `sql.NullString`의 `.Valid` 가드 후 valid이면 문자열, 아니면 `nil`(→JSON `null`)을 응답 map에 넣도록 변경한다. `id`·`nickname` 키와 라우트는 불변.

## 2. 테스트

- [x] 2.1 `auth` 패키지에 `Me` 핸들러 단위 테스트 추가: NULL `avatar_url`·`email` Creator → 응답 JSON에서 두 필드가 `null`인지 검증.
- [x] 2.2 동일 테스트에서 값이 설정된 Creator → 두 필드가 저장 문자열로 노출되는지 검증(회귀 방지).

## 3. 검증

- [x] 3.1 `cd apps/api && go vet ./... && go build ./... && go test ./...` 통과.
- [x] 3.2 실 환경 QA: NULL 필드 Creator JWT로 `GET /api/auth/me`와 `GET /api/creators/me` curl → 두 응답의 `avatar_url`·`email`이 동일하게 `null`인지 확인. 값 있는 Creator로도 회귀 없음 확인.
