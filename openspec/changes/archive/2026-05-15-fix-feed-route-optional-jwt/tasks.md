## 1. 선택적 인증 미들웨어 도입

- [x] 1.1 `apps/api/internal/auth/middleware.go`에 `OptionalJWTMiddleware(jwtSvc *JWTService) func(http.Handler) http.Handler`를 추가한다. 구현 요지: `extractToken(r)`이 빈 문자열을 반환하면 컨텍스트 변경 없이 `next.ServeHTTP(w, r)`. 토큰이 있으면 `jwtSvc.ValidateToken`을 호출하고, 에러나 `uuid.Parse` 실패 시에도 401을 내지 않고 컨텍스트 변경 없이 다음 핸들러를 호출한다. 성공 시에만 `context.WithValue(r.Context(), creatorIDKey, creatorID)`를 세팅한다. design.md Decision 1, Decision 2 근거 주석 포함.
- [x] 1.2 `apps/api/internal/auth/middleware_test.go`(신규 또는 기존 파일에 추가)에 `OptionalJWTMiddleware`의 4개 경로를 검증하는 `func Test...` 테스트 케이스를 추가한다.
  - (a) 토큰 없음 → 핸들러 호출, `CreatorIDFromContext`가 `(uuid.Nil, false)` 반환.
  - (b) 유효한 토큰 쿠키 → 핸들러 호출, `CreatorIDFromContext`가 정상 UUID와 `true` 반환.
  - (c) 유효한 토큰 Authorization Bearer 헤더 → (b)와 동일.
  - (d) 만료·서명 불일치·UUID 파싱 실패 등 잘못된 토큰 → 핸들러 호출(401 아님), `CreatorIDFromContext`가 `(uuid.Nil, false)` 반환.

## 2. `/api/feed` 라우트 wiring

- [x] 2.1 `apps/api/cmd/server/main.go`의 `r.Get("/api/feed", feedHandler.GetFeed)`(L169)를 `r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/api/feed", feedHandler.GetFeed)`로 교체한다.
- [x] 2.2 교체 위치 바로 위에 "spec: feed `피드 라우트는 선택적 인증 미들웨어로 보호된다`" 한 줄 주석을 남겨 회귀 시 의도가 보이도록 한다.

## 3. wiring 회귀 테스트

- [x] 3.1 `apps/api/internal/feed/handler_optional_auth_test.go`를 생성하여 `main.go`의 production wiring(`r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/api/feed", ...)`)을 그대로 재현하는 `chi.Router`를 구성한다. `Handler.GetFeed`가 분기 결정의 상류 신호로 의존하는 `auth.CreatorIDFromContext`를 관찰하는 스파이 핸들러를 라우트에 부착해 다음 두 경우를 검증한다.
  - (a) 토큰 없는 요청 → 핸들러가 호출되고 `CreatorIDFromContext`는 `(uuid.Nil, false)`를 반환(상태 200, 401 아님).
  - (b) 유효한 토큰 쿠키가 있는 요청 → 핸들러가 호출되고 `CreatorIDFromContext`는 토큰 소유자의 UUID와 `true`를 반환.
  - `*sql.DB`·Redis mock 의존성을 새로 도입하지 않기 위해 `Handler.GetFeed`를 직접 호출하지 않고 분기 결정 신호를 관찰하는 스파이로 대체한다. `Handler.GetFeed`의 분기 자체는 동일 패키지의 단위 테스트 책임이며, 본 테스트의 목적은 wiring 회귀 방지로 한정한다.

## 4. 검증

- [x] 4.1 `cd apps/api && go build ./...` 통과.
- [x] 4.2 `cd apps/api && go test ./internal/auth/... ./internal/feed/... ./cmd/server/...` 통과.
- [x] 4.3 `openspec validate fix-feed-route-optional-jwt --strict` 통과.
