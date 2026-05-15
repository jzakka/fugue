## 1. board 라우트 wiring

- [x] 1.1 `apps/api/cmd/server/main.go`의 `r.Get("/", boardsHandler.ListByCreator)`(L156)를 `r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/", boardsHandler.ListByCreator)`로 교체한다.
- [x] 1.2 `apps/api/cmd/server/main.go`의 `r.Get("/{id}", boardsHandler.GetByID)`(L158)를 `r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/{id}", boardsHandler.GetByID)`로 교체한다.
- [x] 1.3 두 교체 위치 바로 위에 "spec: board `공개 보드 조회 라우트는 선택적 인증 미들웨어로 보호된다`" 한 줄 주석을 남겨 회귀 시 의도가 보이도록 한다.

## 2. wiring 회귀 테스트

- [x] 2.1 `apps/api/internal/boards/handler_optional_auth_test.go`(신규)를 생성하여 main.go의 production wiring(`r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get(...)`)을 그대로 재현하는 `chi.Router`를 두 라우트(`/` 및 `/{id}`)에 대해 구성한다. 두 핸들러가 분기 결정의 상류 신호로 의존하는 `auth.CreatorIDFromContext`를 관찰하는 스파이 핸들러를 라우트에 부착해 다음 네 경우를 검증한다.
  - (a) `GET /` 토큰 없는 요청 → 스파이가 호출되고 `CreatorIDFromContext`는 `(uuid.Nil, false)` 반환(상태 200, 401 아님).
  - (b) `GET /` 유효한 토큰 쿠키 요청 → 스파이가 호출되고 `CreatorIDFromContext`는 토큰 소유자의 UUID와 `true` 반환.
  - (c) `GET /{id}` 토큰 없는 요청 → (a)와 동일.
  - (d) `GET /{id}` 유효한 토큰 쿠키 요청 → (b)와 동일.
  - `*sql.DB`·DB mock 의존성을 새로 도입하지 않기 위해 실제 `boards.Handler`를 호출하지 않고 분기 결정 신호를 관찰하는 스파이로 대체한다. `boards.Handler`의 분기 자체는 동일 패키지의 단위 테스트 책임이며, 본 테스트의 목적은 wiring 회귀 방지로 한정한다(직전 사이클의 feed wiring 테스트와 동일 패턴).

## 3. 검증

- [x] 3.1 `cd apps/api && go build ./...` 통과.
- [x] 3.2 `cd apps/api && go test ./internal/auth/... ./internal/boards/... ./cmd/server/...` 통과.
- [x] 3.3 `openspec validate fix-boards-public-get-optional-jwt --strict` 통과.
