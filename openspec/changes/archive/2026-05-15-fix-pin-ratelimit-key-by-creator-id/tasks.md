## Tasks

- [ ] 1. `apps/api/internal/auth/ratelimit.go`에 creator-keyed 미들웨어 surface 추가
  - [ ] 1.1 `Middleware`의 내부 로직을 공통 헬퍼 `serveLimited(w, r, bucketKey)`로 추출. `rateLimitScript`(직전 사이클의 Lua EVAL)와 fail-open 분기는 그대로 재사용.
  - [ ] 1.2 `Middleware`는 헬퍼에 `extractIP(r)`을 버킷 키로 전달(기존 행위와 비트단위 동등).
  - [ ] 1.3 `MiddlewareByCreatorID` 메서드 추가. `auth.CreatorIDFromContext`로 식별자를 꺼내 `"creator:" + uuid.String()`을 버킷 키로 사용. 식별자 부재 시 `"ip:" + extractIP(r)` fallback(spec Decision 2).
  - [ ] 1.4 godoc 두 줄로 두 surface의 사용 시점(per-IP vs per-user)을 명시. spec 인용은 `ratelimit` capability + `docs/architecture.md` 두 곳을 모두 가리킨다.

- [ ] 2. `apps/api/internal/auth/ratelimit_test.go`에 회귀 방지 테스트 4개 추가
  - [ ] 2.1 `TestRateLimiter_MiddlewareByCreatorID_PartitionsByUser`: 같은 IP에서 두 creator가 각각 limit까지 받고, 셋째 시도부터 429를 받는다. miniredis로 키가 `rl:/api/x:creator:<uuid>` 형식인지 검증.
  - [ ] 2.2 `TestRateLimiter_MiddlewareByCreatorID_SharesAcrossIPs`: 같은 creator가 IP 두 곳에서 limit+1번째 시도 시 429를 받는다. 카운터가 IP가 아닌 creator로 분리됨을 확인.
  - [ ] 2.3 `TestRateLimiter_MiddlewareByCreatorID_FallsBackToIPWhenUnauth`: 인증 컨텍스트가 없는 요청은 `ip:` prefix 버킷에 누적되며 limit+1번째에 429를 받는다. fail-open이 발동하지 않음을 확인.
  - [ ] 2.4 `TestRateLimiter_Middleware_IPKeyUnchanged`: 기존 `Middleware`(IP-keyed)의 키 포맷이 `rl:<path>:<ip>`(creator: prefix 없음)를 유지하고, 카운터 분리 행위가 변하지 않음을 확인.
  - [ ] 2.5 기존 6개 fixed-window/fail-open 테스트가 그대로 통과하는지 `go test ./internal/auth/...`로 확인.

- [ ] 3. `apps/api/cmd/server/main.go`의 `/api/pins POST` wiring 교체
  - [ ] 3.1 138행을 `r.With(auth.JWTMiddleware(jwtSvc), pinRL.MiddlewareByCreatorID).Post("/api/pins", pinHandler.Create)`로 변경.
  - [ ] 3.2 137행(`r.Get("/api/pins", pinHandler.List)`) 위에 spec comment 한 줄 추가 — `docs/architecture.md`의 "핀 생성: 30/분/유저" SHALL을 가리키는 인용. 다른 라우트는 변경하지 않는다.

- [ ] 4. `openspec/changes/fix-pin-ratelimit-key-by-creator-id/specs/ratelimit/spec.md`에 ADDED Requirement 1건 작성
  - [ ] 4.1 Requirement 제목: `유저 단위 빈도 제한 surface를 노출한다`.
  - [ ] 4.2 Scenario 4개: (a) 인증 식별자가 있는 요청은 creator-keyed 버킷으로 누적, (b) 같은 creator의 다른 IP 요청도 같은 버킷을 공유, (c) 같은 IP의 다른 creator는 서로 다른 버킷을 가짐, (d) 인증 식별자가 없는 요청은 IP fallback 버킷으로 누적되며 무제한 통과되지 않는다.
  - [ ] 4.3 본 Requirement는 라우트 매트릭스를 규정하지 않으며 그것은 `docs/architecture.md`가 계속 소유한다는 단서를 본문에 포함.

- [ ] 5. 검증·통합
  - [ ] 5.1 `go build ./...` 통과.
  - [ ] 5.2 `go test ./...` 통과(신규 4개 포함).
  - [ ] 5.3 `openspec validate --specs --strict`로 ratelimit capability 검증. 본 change 외 사전 드리프트는 본 change 범위 밖.
  - [ ] 5.4 archive: capability main spec(`openspec/specs/ratelimit/spec.md`)에 신규 Requirement 머지. Purpose는 변경하지 않는다.
