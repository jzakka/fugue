## MODIFIED Requirements

### Requirement: Harvester 노드 단위 통계 정의

시스템은 Harvester가 처리한 한 노드(URL)에 대해 다음 4개 주 카테고리 중 정확히 하나로 집계해야 한다(SHALL): `PinsCreated`(신규 봇 Pin insert), `Deduped`(기존 봇 Pin update), `Skipped`(classifier가 pinnable=false 판정), `Failed`(extractor/upsert 에러). `AdapterFallback`(어댑터 실패로 generic으로 fallback)은 주 카테고리와 독립적인 부가 카운터이며 주 카테고리와 동시에 증가할 수 있다(SHALL). 어댑터 실패가 한 노드 처리에서 발생했다면 generic extractor의 성공/실패와 무관하게 `AdapterFallback`이 1 증가해야 한다(SHALL). ScriptAdapter가 RawItem을 N개 반환하더라도 노드 1개당 주 카테고리 증가는 1이어야 한다(SHALL).

#### Scenario: 신규 페이지 harvest

- **WHEN** Harvester가 새 canonical URL의 페이지를 처리하고 Pin을 새로 insert할 때
- **THEN** PinsCreated가 1 증가한다

#### Scenario: 기존 페이지 재harvest

- **WHEN** Harvester가 이미 봇 Pin이 있는 canonical URL을 다시 처리하고 Pin을 update할 때
- **THEN** Deduped가 1 증가한다

#### Scenario: pinnable=false 페이지 처리

- **WHEN** Harvester가 어떤 페이지를 처리하고 classifier가 pinnable=false로 판정할 때
- **THEN** Skipped가 1 증가한다 (PinsCreated/Deduped/Failed는 증가하지 않는다)

#### Scenario: 추출/upsert 에러

- **WHEN** Harvester가 어떤 페이지에 대해 extractor 또는 upsert에서 에러를 만날 때
- **THEN** Failed가 1 증가한다

#### Scenario: 어댑터 실패 후 generic 성공

- **WHEN** ScriptAdapter가 실패해 generic extractor로 fallback되어 Pin이 생성될 때
- **THEN** PinsCreated 또는 Deduped가 1 증가하고 별도로 AdapterFallback이 1 증가한다

#### Scenario: 어댑터 실패 후 generic 실패

- **WHEN** ScriptAdapter가 실패하고 fallback된 generic extractor에서도 에러가 발생할 때
- **THEN** Failed가 1 증가하고 별도로 AdapterFallback이 1 증가한다

#### Scenario: ScriptAdapter의 N개 RawItem

- **WHEN** ScriptAdapter가 한 노드에 대해 N개의 RawItem을 반환하여 PinDocument 1건으로 축약될 때
- **THEN** 노드 1개당 PinsCreated 또는 Deduped가 정확히 1만 증가한다 (N이 아니다)
