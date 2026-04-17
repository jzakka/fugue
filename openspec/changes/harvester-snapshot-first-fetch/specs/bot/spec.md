## ADDED Requirements

### Requirement: Harvester는 fetch 시 ObjectStorage 스냅샷을 우선 사용한다

Harvester는 노드의 원본 HTML을 가져올 때 `CompositeFetcher` 의미론에 따라 ObjectStorage 스냅샷을 먼저 시도하고, 사용할 수 없을 때만 HTTP fetch로 폴백해야 한다(SHALL). 출처와 무관하게 동일한 바이트열 의미를 갖는 응답을 후속 파싱 파이프라인에 전달해야 한다(SHALL). 참조 의사코드: `apps/api/fuguebot_pseudo.go` 라인 86-97의 `CompositeFetcher.Fetch`.

#### Scenario: 스냅샷 hit 시 네트워크 호출 없이 로컬 파싱
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청하고 ObjectStorage에서 유효한 스냅샷이 반환될 때
- **THEN** Harvester는 외부 사이트로 HTTP 요청을 보내지 않고, ObjectStorage에서 받은 본문만으로 파싱 파이프라인을 진행한다

#### Scenario: 출처 무관한 응답 의미론
- **WHEN** Harvester가 ObjectStorage 또는 HTTP 어느 쪽에서든 본문을 받을 때
- **THEN** 후속 파서/스크립트 실행기는 출처를 구분하지 않고 동일한 원본 HTML 바이트열로 동작한다

---

### Requirement: 스냅샷 사용 불가 시 HTTP fetch로 폴백한다

ObjectStorage 조회가 실패(키 없음, 만료, 저장소 에러 등 모든 케이스)할 경우 Harvester는 동일 노드 URL에 대해 HTTP fetch로 폴백해야 한다(SHALL). 폴백된 HTTP 응답을 ObjectStorage에 재저장할지 여부는 본 요구사항이 정의하지 않으며, 저장 책임은 `pioneer-snapshot-storage` 계약에 위임한다.

#### Scenario: 스냅샷 miss 시 HTTP 폴백
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했으나 ObjectStorage에 해당 스냅샷이 존재하지 않을 때
- **THEN** 동일 URL에 대해 HTTP fetch를 수행하여 본문을 획득한 뒤 파싱 파이프라인을 진행한다

#### Scenario: 스냅샷 expired 시 HTTP 폴백
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했고 ObjectStorage가 TTL 만료로 본문을 반환하지 못할 때
- **THEN** 동일 URL에 대해 HTTP fetch를 수행하여 본문을 획득한 뒤 파싱 파이프라인을 진행한다

#### Scenario: ObjectStorage 에러 시 HTTP 폴백
- **WHEN** Harvester의 ObjectStorage 조회가 네트워크/권한/내부 에러로 실패할 때
- **THEN** Harvester는 즉시 실패로 처리하지 않고 동일 URL에 대해 HTTP fetch로 폴백한다

---

### Requirement: ObjectStorage와 HTTP 모두 실패 시 노드 처리 실패로 집계한다

`CompositeFetcher`의 ObjectStorage 경로와 HTTP 폴백 경로가 모두 본문을 반환하지 못하면, Harvester는 해당 노드 처리를 실패로 분류하고 실행 통계의 `harvest_error_count`를 1 증가시켜야 한다(SHALL). 이 실패는 다른 노드의 처리를 중단시키지 않아야 한다(SHALL).

#### Scenario: 이중 실패 시 실패 카운터 증가
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했고 ObjectStorage 조회와 HTTP 폴백이 모두 본문을 반환하지 못할 때
- **THEN** 해당 노드의 파싱은 수행되지 않고 실행 통계의 `harvest_error_count`가 1 증가한다

#### Scenario: 노드 단위 실패 격리
- **WHEN** 특정 노드의 fetch가 이중 실패로 종료될 때
- **THEN** Harvester는 다음 노드의 처리를 계속 진행하며 단일 노드의 실패가 전체 실행을 중단시키지 않는다
