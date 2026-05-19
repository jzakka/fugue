## ADDED Requirements

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
