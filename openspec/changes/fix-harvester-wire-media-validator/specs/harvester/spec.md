## ADDED Requirements

### Requirement: Harvester 워커 부트스트랩은 미디어 후보 유효성 검증기를 wire한다

production harvester 워커 프로세스의 부트스트랩 경로가 구성하는 Harvester consumer는 미디어 후보 유효성 검증기를 보유한 상태여야 한다(SHALL). 검증기 보유 여부는 외부 코드에서 결정적으로 관찰 가능해야 한다(SHALL). 검증기를 보유하지 않은 consumer가 production 부트스트랩 경로로 구성되어서는 안 된다(SHALL NOT).

본 Requirement는 기존 "미디어 후보 유효성 검증", "정본 키 영속 제한", "검증 실패 사유의 og_data 기록", "Pin primary media invariant" Requirement가 production에서 enforce되도록 보장하는 wiring 계약이다. 검증기 동작 자체의 행위 계약은 위 4개 Requirement가 그대로 규정하며, 본 Requirement는 행위 계약을 변경하지 않는다.

#### Scenario: production 부트스트랩이 wiring을 수행한다

- **WHEN** production harvester 워커 부트스트랩 경로가 Harvester consumer를 구성할 때
- **THEN** 구성된 consumer는 미디어 후보 유효성 검증기를 보유한 상태로 반환된다

#### Scenario: 외부에서 wiring 상태를 관찰할 수 있다

- **WHEN** 호출자가 Harvester consumer 인스턴스의 미디어 후보 유효성 검증기 보유 여부를 조회할 때
- **THEN** 호출자는 내부 필드 접근 없이 wiring 상태를 결정적으로 확인할 수 있다
