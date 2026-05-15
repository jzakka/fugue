# ratelimit Specification

## Purpose

HTTP 요청 빈도 제한 미들웨어가 모든 라우트에 걸쳐 일관되게 보장해야 할 cross-cutting 행위 계약을 규정한다. 본 capability는 라우트별 limit 값·윈도우 길이·적용 매트릭스를 규정하지 않으며 그것들은 `docs/architecture.md`의 Rate Limit 섹션이 소유한다. 본 capability는 그 limit 값이 production에서 의도된 fixed-window 의미로 동작하도록 보장하는 미들웨어 측 invariant만 다룬다.
## Requirements
### Requirement: HTTP 요청 빈도 제한 카운터는 단일 원자 단위로 증가·만료 설정된다

시스템은 HTTP 요청 빈도 제한 미들웨어가 카운터 증가와 윈도우 만료 시점 설정을 단일 원자 단위로 수행해야 한다(SHALL). 한 키의 첫 INCR이 성공한 직후 그 키의 윈도우 TTL이 설정되지 않은 상태가 외부 클라이언트로부터 관측 가능해서는 안 된다(SHALL NOT). 카운터는 첫 INCR 시점에 윈도우 길이만큼의 TTL이 설정되어야 하며, 같은 윈도우 안의 후속 INCR가 TTL을 리셋해서는 안 된다(SHALL NOT). 윈도우 경계가 지나면 카운터는 자연 만료되고 다음 첫 INCR가 윈도우를 다시 시작한다(SHALL).

Redis 명령 실패 시 미들웨어는 fail-open으로 처리하여 요청을 throttle해서는 안 된다(SHALL NOT). limit 값·윈도우 길이·라우트 적용 매트릭스는 본 Requirement의 범위 밖이며 `docs/architecture.md`의 Rate Limit 섹션이 계속 소유한다.

본 Requirement는 `docs/architecture.md`의 "핀 생성: 30/분/유저", "OG fetch: 20/분/IP" SHALL이 production에서 의도된 fixed-window 의미로 enforce되도록 보장하는 미들웨어 계약이다.

#### Scenario: 첫 요청이 카운터를 1로 만들고 윈도우 TTL을 설정

- **WHEN** 한 (key) 에 대한 첫 요청이 미들웨어를 통과하면
- **THEN** 카운터는 1이 되고, 같은 원자 단위에서 그 key의 TTL이 윈도우 길이로 설정된다. 외부 관측자는 카운터가 1이면서 TTL이 음수인 상태를 볼 수 없다

#### Scenario: 같은 윈도우 안의 후속 요청은 카운터만 증가시킨다

- **WHEN** 윈도우 안에서 같은 key에 대한 두 번째 이후 요청이 미들웨어를 통과하면
- **THEN** 카운터는 증가하지만 TTL은 첫 INCR이 설정한 값에서 자연 감소만 한다. TTL이 윈도우 길이로 리셋되지 않는다

#### Scenario: 윈도우 경과 후 첫 요청은 새 윈도우를 시작한다

- **WHEN** 윈도우 길이만큼의 시간이 지나 key가 자연 만료된 뒤 같은 key에 대한 요청이 들어오면
- **THEN** 카운터는 1이 되고 TTL이 윈도우 길이로 다시 설정된다. 이전 윈도우의 카운터는 새 윈도우로 이월되지 않는다

#### Scenario: limit 초과 후 윈도우가 끝나기 전에는 throttle 유지

- **WHEN** 한 key의 카운터가 limit를 초과한 뒤 같은 윈도우 안에서 후속 요청이 들어오면
- **THEN** 미들웨어는 `Retry-After` 헤더와 함께 429 응답을 반환한다

#### Scenario: Redis 명령 실패 시 fail-open

- **WHEN** Redis 명령 실패로 카운터를 관측할 수 없으면
- **THEN** 미들웨어는 요청을 throttle하지 않고 다음 핸들러를 호출한다

### Requirement: 유저 단위 빈도 제한 surface를 노출한다

시스템은 HTTP 요청 빈도 제한 미들웨어가 두 종류의 키 단위 surface를 노출해야 한다(SHALL). 하나는 클라이언트 IP를 카운터 버킷 키로 사용하는 기존 surface이고, 다른 하나는 요청 컨텍스트의 인증된 유저 식별자를 카운터 버킷 키로 사용하는 surface다. 두 surface는 같은 미들웨어가 보장해야 할 fixed-window 원자성·fail-open invariant(`Requirement: HTTP 요청 빈도 제한 카운터는 단일 원자 단위로 증가·만료 설정된다`)를 그대로 공유한다.

유저 단위 surface는 요청 컨텍스트에서 인증된 유저 식별자가 관측되면 그 식별자만으로 카운터 버킷을 분리해야 한다(SHALL). 같은 식별자에 대한 요청은 클라이언트 IP가 달라도 같은 버킷을 공유해야 하고(SHALL), 같은 IP에서 발생한 서로 다른 식별자의 요청은 서로 다른 버킷을 가져야 한다(SHALL). 유저 단위 surface에 인증 식별자가 부재한 요청이 도달하면(상위 인증 미들웨어 누락 같은 wiring 사고) 미들웨어는 클라이언트 IP를 fallback 키로 사용해 카운터를 분리해야 하며, 인증 식별자 부재만을 사유로 요청을 무제한 통과시키지 말아야 한다(SHALL NOT).

본 Requirement는 어느 라우트가 어느 surface를 사용해야 하는지(라우트 적용 매트릭스)를 규정하지 않는다. 그것은 `docs/architecture.md`의 Rate Limit 섹션이 계속 소유한다. 본 Requirement는 그 섹션의 "핀 생성: 30/분/유저"·"OG fetch: 20/분/IP" SHALL을 production에서 enforce할 수 있도록 미들웨어가 두 단위(per-IP, per-user)의 키를 모두 표현 가능하게 만드는 surface 계약이다.

#### Scenario: 인증 식별자가 있는 요청은 유저 단위 버킷으로 누적

- **WHEN** 인증된 유저의 요청이 유저 단위 surface를 통과하면
- **THEN** 그 요청의 카운터는 클라이언트 IP가 아닌 그 유저의 식별자를 키로 하는 버킷에 누적된다

#### Scenario: 같은 유저는 IP가 달라도 같은 버킷을 공유

- **WHEN** 같은 인증 식별자가 두 개의 서로 다른 클라이언트 IP에서 유저 단위 surface로 요청을 보내면
- **THEN** 두 IP의 요청 카운트는 같은 버킷에 누적되어 한 윈도우 안의 limit를 통합하여 적용받는다

#### Scenario: 같은 IP의 두 유저는 서로 다른 버킷을 가진다

- **WHEN** 같은 클라이언트 IP에서 두 개의 서로 다른 인증 식별자가 유저 단위 surface로 요청을 보내면
- **THEN** 두 유저의 카운트는 서로 분리된 버킷에 누적되어 한 유저의 limit 초과가 다른 유저의 요청을 차단하지 않는다

#### Scenario: 인증 식별자가 부재한 요청은 IP fallback 버킷으로 누적

- **WHEN** 유저 단위 surface에 인증 식별자가 부재한 요청이 도달하면
- **THEN** 미들웨어는 클라이언트 IP를 키로 하는 fallback 버킷에 카운트를 누적하며, 인증 부재만을 사유로 요청을 무제한 통과시키지 않는다

