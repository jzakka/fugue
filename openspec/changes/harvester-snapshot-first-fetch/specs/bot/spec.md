## MODIFIED Requirements

### Requirement: Harvester가 실제 HTML을 가져온다

시스템은 Harvester가 크롤 그래프의 노드 URL에서 실제 HTML을 가져올 수 있어야 한다(SHALL). Harvester는 단일 `Fetcher` 인터페이스(`Fetch(url) ([]byte, error)`)를 통해 HTML을 가져오며, 구현체로 **ObjectStorage 우선 → HTTP fallback** 합성을 수행하는 `CompositeFetcher`를 사용해야 한다(SHALL). Fetcher가 반환하는 바이트열은 출처(스냅샷/HTTP)와 무관하게 동일한 의미의 원본 HTML로 후속 파싱 파이프라인에 전달되어야 한다(SHALL). 참조 의사코드: `apps/api/fuguebot_pseudo.go` 라인 86-97의 `CompositeFetcher.Fetch`.

ObjectStorage 조회 시 사용하는 스냅샷 키 포맷과 해시 함수(sha256)는 본 capability에서 자체 정의하지 않고 **`pioneer-snapshot-storage` capability의 키 규약 및 공용 키 빌더 함수를 그대로 따른다**(SHALL). Harvester 측에서 키 포맷·해시 함수를 재구현해서는 안 된다(MUST NOT).

#### Scenario: 스냅샷 hit 시 네트워크 호출 없이 로컬 파싱
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청하고 ObjectStorage에서 유효한 스냅샷이 반환될 때
- **THEN** Harvester는 외부 사이트로 HTTP 요청을 보내지 않고, ObjectStorage에서 받은 본문만으로 파싱 파이프라인을 진행한다

#### Scenario: 출처 무관한 응답 의미론
- **WHEN** Harvester가 ObjectStorage 또는 HTTP 어느 쪽에서든 본문을 받을 때
- **THEN** 후속 파서/스크립트 실행기는 출처를 구분하지 않고 동일한 원본 HTML 바이트열(gzip 해제 완료)로 동작한다

#### Scenario: Pioneer와 Harvester의 fetch 로직 공유
- **WHEN** Pioneer와 Harvester가 HTML을 가져올 때
- **THEN** 동일한 공유 `Fetcher` 인터페이스 및 HTTP 설정(사이즈 제한, 리다이렉트 제한, 타임아웃, User-Agent)을 사용하여 중복 구현을 방지한다

#### Scenario: 스냅샷 키 규약은 pioneer-snapshot-storage를 참조한다
- **WHEN** Harvester의 ObjectStorage 조회가 스냅샷 키를 계산할 때
- **THEN** `pioneer-snapshot-storage` capability가 정의한 키 빌더(normalized URL의 sha256 해시 기반)를 그대로 사용하며, 본 capability에서 키 포맷을 별도로 정의하지 않는다

---

### Requirement: 스냅샷 사용 불가 시 HTTP fetch로 폴백한다

ObjectStorage 조회가 실패하는 **모든 케이스(키 없음, TTL 만료, 네트워크 에러, 권한 에러, 내부 5xx 에러 등)** 를 Harvester는 단일 "miss"로 취급하여 동일 노드 URL에 대해 HTTP fetch로 폴백해야 한다(SHALL). 실패 유형에 따라 fetch 동작이 달라져서는 안 된다(MUST NOT). 폴백된 HTTP 응답을 ObjectStorage에 재저장할지 여부는 본 요구사항이 정의하지 않으며, 저장 책임은 `pioneer-snapshot-storage` 계약에 위임한다.

ObjectStorage 실패 유형 구분은 **로그/메트릭 레벨에서만** 수행되어야 하며(운영 관찰·알람 임계치 산정용), fetch 의사결정(fallback 여부)에는 영향을 주지 않는다(SHALL). 이는 동작(behavior)이 아니라 관측(observability)의 영역이다.

#### Scenario: 스냅샷 miss 시 HTTP 폴백
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했으나 ObjectStorage에 해당 스냅샷이 존재하지 않을 때
- **THEN** 동일 URL에 대해 HTTP fetch를 수행하여 본문을 획득한 뒤 파싱 파이프라인을 진행한다

#### Scenario: 스냅샷 expired 시 HTTP 폴백
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했고 ObjectStorage가 TTL 만료(365일 lifecycle rule에 의한 자동 삭제 포함)로 본문을 반환하지 못할 때
- **THEN** 동일 URL에 대해 HTTP fetch를 수행하여 본문을 획득한 뒤 파싱 파이프라인을 진행한다

#### Scenario: ObjectStorage 에러 시 HTTP 폴백
- **WHEN** Harvester의 ObjectStorage 조회가 네트워크/권한/내부(5xx) 에러로 실패할 때
- **THEN** Harvester는 즉시 실패로 처리하지 않고 동일 URL에 대해 HTTP fetch로 폴백한다

#### Scenario: 실패 유형은 로그로만 구분된다
- **WHEN** ObjectStorage 조회가 실패할 때
- **THEN** Fetcher는 실패 원인(not_found / expired / network / permission / internal)을 로그·메트릭으로 기록하지만, fetch 동작은 모든 케이스에서 동일하게 HTTP fallback으로 수렴한다

---

### Requirement: ObjectStorage와 HTTP 모두 실패 시 노드 처리 실패로 집계한다

`CompositeFetcher`의 ObjectStorage 경로와 HTTP 폴백 경로가 모두 본문을 반환하지 못하면, Harvester는 해당 노드 처리를 실패로 분류하고 실행 통계의 `harvest_error_count`를 1 증가시켜야 한다(SHALL). 이 실패는 다른 노드의 처리를 중단시키지 않아야 한다(SHALL). 단일 노드 실패가 Harvester 실행 전체를 중단시켜서는 안 된다(MUST NOT).

#### Scenario: 이중 실패 시 실패 카운터 증가
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했고 ObjectStorage 조회와 HTTP 폴백이 모두 본문을 반환하지 못할 때
- **THEN** 해당 노드의 파싱은 수행되지 않고 실행 통계의 `harvest_error_count`가 1 증가한다

#### Scenario: 노드 단위 실패 격리
- **WHEN** 특정 노드의 fetch가 이중 실패로 종료될 때
- **THEN** Harvester는 다음 노드의 처리를 계속 진행하며 단일 노드의 실패가 전체 실행을 중단시키지 않는다

#### Scenario: fetch 출처 및 실패 종류의 관측성
- **WHEN** Harvester가 fetch를 수행할 때
- **THEN** 각 호출의 fetch 출처(snapshot/http) 및 실패 시 ObjectStorage 에러 종류가 로그/메트릭으로 식별 가능하다(운영 관찰용이며 fetch behavior에는 영향을 주지 않는다)
