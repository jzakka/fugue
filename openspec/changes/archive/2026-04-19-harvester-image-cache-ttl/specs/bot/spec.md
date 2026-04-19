## ADDED Requirements

### Requirement: 캐시된 primary 이미지 객체는 설정 가능한 TTL 후 만료 대상이 된다
시스템은 이미지 캐시 네임스페이스에 저장된 primary 이미지 객체에 대해 **연령 기반 TTL**을 capability 내 계약으로 정의해야 한다(SHALL). 각 캐시 객체는 자신의 작성/최종 쓰기 시점으로부터 TTL이 경과한 시점부터 시스템에 의해 **제거 대상(eligible for removal)** 상태가 되어야 하며(SHALL), TTL 미경과 시점에서는 제거 대상이 아니어야 한다(SHALL NOT remove before TTL). TTL 값은 운영자가 설정 가능해야 한다(SHALL be configurable).

본 requirement는 **primary 이미지 캐시 네임스페이스 한정**이다. 본문 미디어(item의 media 본체) 저장 네임스페이스의 만료 정책은 본 requirement의 범위가 아니다. 제거 대상 여부의 판정 근거와 실제 제거를 수행하는 메커니즘, TTL의 기본값과 설정 키 이름은 **내부 구현 세부**이며 design 문서에서 확정한다.

캐시 객체의 만료는 Pin 생성 경로와 **비동기**이다. 만료 처리의 성공/실패는 Pin 생성의 성공/실패에 영향을 주지 않아야 한다(SHALL NOT block Pin creation). 구체적으로, 기존의 이미지 캐시 실패 fallback 동작(다운로드/업로드/용량 초과 시 원본 후보 URL로 기록), 후보가 없을 때 공란으로 남는 동작, 캐시 성공 시점의 관찰 가능 결과는 TTL 설정 여부 및 만료 처리 상태와 **무관하게 보전**되어야 한다(SHALL preserve existing cache-path observable behavior). 만료로 인한 참조 해소 실패 가능성은 Pin 이후 조회 시점의 사후 현상이며, 그 참조의 해소 결과(예: 404)는 본 capability의 실패로 간주하지 않는다. 이 경우 Pin 자체는 유효하며, 소비자 측 UX가 참조 해소 실패를 허용해야 한다.

동일 후보 URL의 재캐시가 별도 객체로 저장된다는 기존 Requirement("이미지 캐시 객체는 후보 URL에서 파생된 안정적이고 충돌 회피된 키로 저장된다")는 유지된다. 따라서 만료는 **객체 단위**로 평가되며, 같은 후보 URL의 여러 객체가 각각 자신의 작성/최종 쓰기 시점 기준 TTL에 따라 독립적으로 제거 대상이 된다.

#### Scenario: TTL 미경과 객체는 제거 대상이 아니다
- **WHEN** 이미지 캐시 객체의 작성/최종 쓰기 시점으로부터 TTL이 아직 경과하지 않은 시점에 시스템이 만료 판정을 수행할 때
- **THEN** 해당 객체는 제거 대상이 아니며 storage에서 제거되지 않는다

#### Scenario: TTL 경과 객체는 제거 대상이 된다
- **WHEN** 이미지 캐시 객체의 작성/최종 쓰기 시점으로부터 TTL이 경과한 이후 시점에 시스템이 만료 판정을 수행할 때
- **THEN** 해당 객체는 제거 대상으로 분류되고, 시스템의 제거 메커니즘에 의해 storage에서 삭제된다

#### Scenario: TTL 값은 설정 가능하다
- **WHEN** 운영자가 TTL 설정 값을 기본값과 다른 값으로 지정할 때
- **THEN** 모든 이후 캐시 객체의 만료 판정은 지정된 TTL 값을 기준으로 수행된다

#### Scenario: 만료 처리는 Pin 생성을 막지 않는다
- **WHEN** 만료 처리가 동시에 실행 중이거나 실패한 상태에서 Harvester가 새 Pin을 생성할 때
- **THEN** Pin 생성은 만료 처리의 상태와 무관하게 기존 이미지 캐시 Requirement의 성공/실패 기준에 따라 독립적으로 처리된다

#### Scenario: 만료된 객체 참조는 capability 실패가 아니다
- **WHEN** Pin의 대표 이미지 참조 속성이 TTL 경과 이후 시점에 소비자 조회 시 해소되지 않을 때
- **THEN** 본 capability의 이미지 캐시 Requirement는 여전히 "Pin 생성 시점"의 성공 기준으로 판정되며, 사후 해소 실패는 본 capability의 실패로 집계되지 않는다

#### Scenario: 동일 후보 URL의 두 객체는 각자 TTL을 가진다
- **WHEN** 같은 후보 URL이 시점 T1과 T2(T2 > T1)에 각각 별도 객체로 캐시되고 T1의 객체만 TTL이 경과한 시점에 판정할 때
- **THEN** T1 시점에 저장된 객체만 제거 대상이 되고 T2 시점에 저장된 객체는 제거 대상이 아니다

#### Scenario: 만료는 primary 이미지 캐시 네임스페이스에만 적용된다
- **WHEN** 시스템이 본 capability의 TTL 만료 판정을 수행할 때
- **THEN** 본문 미디어(item의 media 본체) 저장 네임스페이스의 객체는 판정 대상에 포함되지 않는다

## REMOVED Requirements

### Requirement: 이미지 캐시 객체의 TTL/만료는 본 capability 외부다
**Reason**: 본 change에서 TTL/만료를 capability 내부 계약으로 끌어와 정의한다. 이전 Requirement는 "capability가 만료를 정의하지 않는다"는 **부정 계약**을 규정했으나, 무기한 누적으로 인한 스토리지 비용 상한이 제품 내에서 보장되지 않는 문제가 있어 새로운 ADDED Requirement(캐시된 primary 이미지 객체는 설정 가능한 TTL 후 만료 대상이 된다)로 대체한다.
**Migration**: 해당 Requirement가 보장하던 "본 spec의 관찰 가능 동작(캐시 성공/실패 fallback, 후보 없음 공란)은 lifecycle 설정 여부에 따라 변하지 않는다"는 보전 약속은, 신규 Requirement 본문의 "기존의 이미지 캐시 실패 fallback 동작, 후보가 없을 때 공란으로 남는 동작, 캐시 성공 시점의 관찰 가능 결과는 TTL 설정 여부 및 만료 처리 상태와 무관하게 보전되어야 한다"는 SHALL 조항으로 승계된다. 기존 "이미지 캐시 실패는 단일 fallback 경로로 처리된다" Requirement는 본 change에서 변경되지 않으므로 Pin 생성 경로의 관찰 가능 동작은 유지된다. 운영자가 별도 bucket lifecycle 설정을 두고 있었다면, 본 change 이후 capability 내 TTL 설정과 중복되지 않도록 design 문서의 구현 결정에 따라 정리한다.
