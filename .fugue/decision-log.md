# Decision Log

루프가 머지에 성공한 변경, 사용자가 명시적으로 내린 결정, 사용자가 반려한 변경의 이유를 누적하는 곳. 루프는 매 사이클 시작 시 마지막 10개를 읽고, 같은 결정을 다시 침범하지 않는다.

## 작성 규칙

- 한 항목 = 1~3줄.
- 형식:
  ```
  ## YYYY-MM-DD — [track] 짧은 제목
  결정/변경: 무엇을. (필요하면 PR/OpenSpec change id)
  이유: 왜. (디자인/스펙/사용자 결정 인용)
  영향 범위: 어디까지 적용되며 어디는 적용되지 않는가.
  ```
- track: `design` | `system` | `user`(사용자 직접 결정) | `reject`(반려된 변경)
- 시간순 누적. 위가 최신.

## 항목

## 2026-05-15 — [system] Pioneer 부트스트랩이 RobotsFilter에 HostRateLimiter 인스턴스를 wire하도록 교체
결정/변경: `apps/api/cmd/bot/main.go`의 `runPioneerConsumer`가 `buildHostRateLimiter(config.LoadSchedulerHostConfig())` 결과를 지역 변수 `rl`로 추출해 `sched.WithRateLimiter(rl)`와 신규 `buildPioneerConsumer(sched, store, rl)` 두 곳에 동일 인스턴스를 전달하도록 변경(`bot.NewRobotsFilter(nil)` 직접 호출 제거). `apps/api/cmd/bot/pioneer_consumer_builder.go` 신규(`(*bot.PioneerConsumer, *bot.RobotsFilter)` 반환으로 결정적 wiring 검증 surface 노출). `apps/api/internal/bot/robots_filter.go`에 read-only accessor `RateSetter() HostRateSetter` 1개 추가(시그니처·인터페이스·nil 허용 계약 미변경). OpenSpec change `fix-pioneer-robots-filter-host-rate-setter-wiring` 머지(아카이브 `2026-05-15-fix-pioneer-robots-filter-host-rate-setter-wiring`), `bot` capability에 보조 Requirement `Pioneer 부트스트랩은 RobotsFilter에 HostRateLimiter를 wire한다`(2 Scenarios) 추가.
이유: 기존 Requirement `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다`(L750-768)의 SHALL과 Scenario "Crawl-delay 파싱 및 호스트 rate 갱신"이 production pioneer 워커에서 enforce되지 않았다. `runPioneerConsumer`가 `bot.NewRobotsFilter(nil)`을 호출해 RobotsFilter의 `rateSetter == nil` 가드 분기로 빠지면서 SetHostRate가 한 번도 호출되지 않아, robots.txt에 `Crawl-delay: N`이 명시된 호스트도 scheduler의 기본 bucket(1 req/sec, burst 5)을 그대로 사용했다. `scheduler` capability의 `robots.txt Crawl-delay를 호스트 rate로 반영한다`(L297-306)는 호출 책임을 `pioneer-link-filter-policy`에 할당하므로 본 결함은 pioneer 부트스트랩의 wiring 결함으로 한정된다.
영향 범위: `runPioneerConsumer`/`buildPioneerConsumer` 한 경로만 wiring 교체. harvester 부트스트랩(`buildHarvesterConsumer`)·`SetHostRate`의 입력 검증·`HostRateLimiter` 동작·robots.txt 캐시 TTL 24h·single-flight 정책은 변경하지 않음. `NewRobotsFilter`의 nil rateSetter 허용 계약은 기존 단위 테스트 호환을 위해 보존(design Non-Goals D4). 2개 cmd/bot 회귀 단위 테스트(fake `HostRateSetter`로 동일 인스턴스 전파·nil 미허용 wiring 검증). 진행 중 별개 change `fix-scheduler-host-rate-limiter-config-wiring`(`buildHostRateLimiter`에 Config 전달 영역)와는 범위가 분리되어 충돌 없음.

## 2026-05-15 — [system] 핀 조회·핀 생성·보드 추가 핸들러에 interaction piggyback 기록 wiring
결정/변경: `apps/api/internal/interaction/recorder.go` 신규 — `Recorder` 인터페이스(`CreateInteraction`만 노출, `*db.Queries`가 자동 만족) + `Record(ctx, r, userID, pinID, kind)` best-effort 헬퍼(잘못된 type은 INSERT 전 short-circuit, DB 에러는 log만). `pin.PinQuerier` 인터페이스에 `CreateInteraction` 추가, `pin.Create`(인증 보장 후)와 `pin.GetByID`(인증 분기 후) 응답 직전 `interaction.Record(...)` 호출. `boards.AddPin`(인증 보장 후) 동일 호출. `apps/api/cmd/server/main.go`의 `GET /api/pins/{id}` 라우트에 `auth.OptionalJWTMiddleware` 부착(미인증 401 회피 + 인증 context 노출 동시 달성). OpenSpec change `fix-interaction-piggyback-on-pin-and-board-actions` 머지(아카이브 `2026-05-15-fix-interaction-piggyback-on-pin-and-board-actions`), `interaction` capability에 Requirement `시스템은 핀 조회·핀 생성·보드 추가 핸들러 진입 시 인증된 호출자에 한해 interaction 행동을 best-effort로 기록한다`(3 Scenarios) 추가. archive 진행 중 main `interaction/spec.md`의 사전 드리프트(`## ADDED Requirements`로 시작, `## Purpose`/`## Requirements` 누락) 최소 복구 동반.
이유: 기존 Requirement `유저 행동을 기록한다`의 4개 Scenarios SHALL("조회/핀/보드추가 시 기록", "원래 요청은 정상 처리")이 production에서 enforce되지 않았다. `CreateInteraction`의 유일한 호출자는 별도 endpoint `POST /api/interactions`(`internal/interaction/handler.go:55`)였고, pin/board 핸들러 어디서도 호출하지 않았다. AGENTS.md "interaction: 암묵적 행동 기록"과 spec "유저의 원래 요청(조회, 핀, 보드 추가)은 정상적으로 처리된다" 표현이 일관되게 piggyback 모델을 가정. design.md Decision 1로 동기 best-effort INSERT 모델 채택(architecture.md의 비동기 이벤트 파이프라인은 본 결함의 최소 해결 범위 밖).
영향 범위: `GET /api/pins/{id}`·`POST /api/pins`·`POST /api/boards/{id}/pins` 세 라우트만 piggyback wiring. 별도 endpoint `POST /api/interactions`는 변경하지 않음(Decision 6). `interactions` 테이블 스키마·인덱스·sqlc 쿼리·`db.Queries` 코드 변경 없음. 미인증 호출자는 `OptionalJWTMiddleware`로 401을 받지 않으면서 동시에 piggyback 분기에도 진입하지 않음(Scenario "미인증 유저는 기록하지 않는다" 보존). best-effort라 DB 에러가 원래 요청 응답을 변경하지 않음(Scenario "기록 실패가 유저 경험에 영향을 주지 않는다" 보존). 4개 헬퍼 단위 테스트(happy path·3 known types·DB error best-effort·invalid type short-circuit). 핸들러 측 통합 테스트는 별도 surface refactor가 필요해 후속 change로 미룸(design.md Decision 7).

## 2026-05-15 — [system] 개인화 피드 페이지네이션이 페이지 간 작품 중복 반환하던 버그 수정
결정/변경: `apps/api/db/queries/interactions.sql`의 `RecommendByTags`·`RecommendByMediaType`에 OFFSET 파라미터와 `p.id DESC` tiebreaker 추가, sqlc 재생성. `apps/api/internal/feed/handler.go` `buildPersonalizedFeed`가 cursor의 페이지 offset을 태그 추천·미디어 타입 추천·최신 보충·fill-gap 네 갈래 모두에 일관되게 전파. 테스트 가능성 확보를 위해 `FeedQuerier` 인터페이스 + `NewHandlerWithQuerier` 생성자 도입, `auth.WithCreatorID` 공개 헬퍼 추가. OpenSpec change `fix-feed-personalized-pagination-no-cross-page-duplicates` 머지(아카이브 `2026-05-15-fix-feed-personalized-pagination-no-cross-page-duplicates`), `feed` capability에 Requirement `개인화 피드의 페이지네이션은 페이지 간 작품 중복을 반환하지 않는다`(3 Scenarios) 추가.
이유: 기존 Requirement `개인화된 추천 피드를 제공한다` Scenario "피드 페이지네이션" THEN "이전 페이지에 포함되지 않은 작품이 반환된다" SHALL이 production에서 enforce되지 않았다. 개인화 분기의 `RecommendByTags`·`RecommendByMediaType`는 OFFSET 파라미터 자체가 없었고 최신 보충의 `ListPinsWithCreator`도 `Offset: 0` 하드코딩, fill-gap도 페이지 offset이 아닌 `len(latestRows)`만 사용해 페이지 1과 2가 동일한 상위 결과를 반환했다. design.md Decision 1로 cursor 모델은 기존 offset 기반을 유지하고 underlying 쿼리 전파만 수정.
영향 범위: 개인화 분기(authenticated + pinCount >= 콜드스타트 임계)에만 wiring 변경. cold-start/비인증 분기는 기존 `buildLatestFeed` 동작 그대로 유지. cache 키 포맷·cursor payload 포맷은 변경하지 않음(Decision 5). within-page 중복(추천 후보와 최신 후보 간 같은 id 등장) 제거는 본 change 범위 밖이며 별도 backlog 후보로 둠(Decision 4). 3개 페이지네이션 단위 테스트(페이지 간 disjoint·offset 전파·media-type fallback offset 전파)와 기존 feed 테스트 회귀 없음.

## 2026-05-15 — [system] 공개 프로필 응답에 보드·핀 요약 포함
결정/변경: `apps/api/internal/creator/dto.go` `CreatorPublicDTO`에 `boards`/`pins` 키 추가, `handler.go` `GetByID`가 `ListPublicBoardsByCreatorLimited`(신규 sqlc 쿼리, LIMIT=20)와 기존 `ListPinsByCreator`(LIMIT=12)를 추가 호출해 페이로드를 채우도록 wiring. OpenSpec change `fix-creator-public-profile-include-boards-pins` 머지(아카이브 `2026-05-15-fix-creator-public-profile-include-boards-pins`), `profile` capability에 Requirement `공개 프로필 조회 응답에 보드 요약과 핀 요약을 포함한다`(5 Scenarios) 추가. archive 중 `openspec/specs/profile/spec.md`의 `## ADDED Requirements` 사전 드리프트를 `## Purpose` + `## Requirements` 정상 헤더로 1회 보수.
이유: 기존 Scenario "공개 프로필 조회" THEN 절이 "닉네임, 아바타, 보드 목록, 핀 목록이 반환된다"를 명시하지만 `CreatorPublicDTO`는 5개 스칼라 필드만 가져 `보드 목록`·`핀 목록` SHALL이 production에서 enforce되지 않았다. 별도 엔드포인트 분리 방식은 spec text가 "반환된다"로 묶고 있어 채택하지 않고, 응답 페이로드에 요약 두 배열을 함께 포함하는 방향을 design.md Decision 1·5로 확정.
영향 범위: `GET /api/creators/{id}` 응답만 변경. `boards`는 호출자 인증 여부와 무관하게 공개 보드만 포함(비공개 보드는 `GET /api/boards?creator_id=...` 본인 분기 책임), `pins`는 기존 `ListPinsByCreator`를 LIMIT=12로 재사용해 sqlc 쿼리 확산 억제. 상한 20/12는 spec이 아닌 `handler.go` 패키지 상수로 둠. `GetMe`/`UpdateMe`는 변경하지 않음. 6개 단위 테스트(응답 shape·빈 배열 직렬화·LIMIT 전파·boards 500·pins 500·404 skip)로 회귀 방지.

## 2026-05-15 — [system] /api/pins 빈도 제한을 유저 단위 surface로 교체
결정/변경: `apps/api/internal/auth/ratelimit.go`에 `MiddlewareByCreatorID` surface를 추가하고(공유 `middleware(next, bucketKeyFn)` 헬퍼로 fixed-window 원자성 invariant 그대로 공유), `apps/api/cmd/server/main.go:138`의 `/api/pins POST` wiring을 `pinRL.Middleware` → `pinRL.MiddlewareByCreatorID`로 교체. OpenSpec change `fix-pin-ratelimit-key-by-creator-id` 머지(아카이브 `2026-05-15-fix-pin-ratelimit-key-by-creator-id`), 기존 `ratelimit` capability에 Requirement `유저 단위 빈도 제한 surface를 노출한다` 1건 추가.
이유: `docs/architecture.md`의 SHALL "핀 생성: 30/분/유저"는 enforce 단위를 명시하지만 기존 미들웨어가 `path+IP`만으로 키를 만들어 (a) 공유 NAT 뒤 다른 유저들이 한 명의 30/min 쿼터를 공유, (b) 한 유저가 IP를 바꿔 per-user 상한을 우회하는 두 위반이 발생했다. 직전 cycle의 ratelimit 원자성 invariant는 "라우트 적용 매트릭스는 본 Requirement의 범위 밖"이라고 명시하므로 본 결함은 별개 surface 결함.
영향 범위: `/api/pins POST` 라우트만 surface 교체. `/api/og/fetch`(per-IP 의도), `/api/auth/{provider}/login`·`/api/auth/{provider}/callback`·`/api/auth/logout`(per-IP 의도)은 변경하지 않음. unauth context fallback은 IP 키로 카운트하며 fail-open 하지 않음. miniredis 기반 단위 테스트 4개로 creator 분리·IP 공유·unauth fallback·IP surface 회귀 방지. archive 진행 중 `interaction/pin/profile/spec.md`의 사전 드리프트는 본 change 범위 밖.

## 2026-05-15 — [system] RateLimiter INCR+EXPIRE 원자화
결정/변경: `apps/api/internal/auth/ratelimit.go`의 두 왕복 INCR+EXPIRE 패턴을 `redis.NewScript`의 단일 Lua EVAL로 교체. OpenSpec change `fix-ratelimit-incr-expire-atomicity` 머지(아카이브 `2026-05-15-fix-ratelimit-incr-expire-atomicity`), 신규 capability `ratelimit` 추가.
이유: INCR 성공 후 EXPIRE 왕복만 실패하면 키가 TTL=-1로 영구 잔존해 후속 요청이 count를 누적, `count > limit` 이후 그 (IP, path) 쌍이 무한 429를 받는다. `docs/architecture.md`의 "핀 생성: 30/분/유저", "OG fetch: 20/분/IP" SHALL은 fixed-window 카운터 리셋을 전제하므로 영구 throttle은 doc 위반.
영향 범위: `RateLimiter.Middleware` 본문만 교체. limit 값·윈도우 길이·라우트 적용 매트릭스·fail-open 정책·키 포맷·`extractIP`는 변경하지 않음. miniredis 기반 단위 테스트 6개로 fixed-window·fail-open·키 분리 회귀 방지. archive 진행 중 `interaction/pin/profile/spec.md`의 사전 드리프트는 본 change 범위 밖.

## 2026-05-15 — [system] /api/boards·/api/boards/{id} 선택적 JWT 미들웨어 wiring
결정/변경: `auth.OptionalJWTMiddleware`를 `apps/api/cmd/server/main.go`의 `GET /api/boards`·`GET /api/boards/{id}` 두 라우트에 부착. OpenSpec change `fix-boards-public-get-optional-jwt` 머지(아카이브 `2026-05-15-fix-boards-public-get-optional-jwt`).
이유: 두 라우트는 핸들러 본문에서 `auth.CreatorIDFromContext`로 owner 분기를 수행하지만 미들웨어 미부착으로 항상 비인증 분기에 진입했다. board spec `보드를 조회한다` Scenario "소유자는 비공개 보드 조회 가능"·`유저의 보드 목록을 조회한다` Scenario "본인의 보드 목록" 두 SHALL이 production에서 enforce되지 않았다.
영향 범위: GET 두 라우트만 부착. 기존 JWT 보호 라우트(Create/Update/Delete/AddPin/RemovePin)는 변경하지 않음. archive 진행 중 `openspec/specs/board/spec.md`의 헤더 손상(`## ADDED Requirements`, `## Purpose` 누락) 최소 복구 동반.

## 2026-05-15 — [system] /api/feed 선택적 JWT 미들웨어 wiring
결정/변경: `OptionalJWTMiddleware`를 `apps/api/internal/auth/middleware.go`에 신설하고 `apps/api/cmd/server/main.go`의 `/api/feed` 라우트에 부착. OpenSpec change `fix-feed-route-optional-jwt` 머지(아카이브 `2026-05-15-fix-feed-route-optional-jwt`).
이유: 기존 `JWTMiddleware`는 토큰 부재 시 401을 반환하므로 `/api/feed`에 부착할 수 없었고, 미부착 상태에서는 `auth.CreatorIDFromContext`가 항상 false를 반환해 feed spec `개인화된 추천 피드를 제공한다`의 인증 시나리오 두 개가 production에서 enforce되지 않았다. 토큰 만료 통지(X-Token-Expired)는 본 change 범위에서 제외.
영향 범위: `/api/feed`에만 부착. 다른 라우트에는 적용하지 않음. archive 진행 중 main `openspec/specs/auth/spec.md`·`feed/spec.md`의 헤더 손상(`## ADDED Requirements` 잔류, `## Purpose` 누락)이 archive를 막아 archive 전제로 두 main spec 최소 복구(`## Purpose` 1줄 추가, 헤더 교정)를 동반 수행.
