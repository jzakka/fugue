## ADDED Requirements

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
