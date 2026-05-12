## MODIFIED Requirements

<!--
본 MODIFIED Requirement 블록은 OpenSpec delta 의미론에 따라 기존 `bot` capability의 동명 Requirement 블록 전체(Requirement 본문 + 기존 Scenario 2건)를 아래 텍스트로 교체한다. 교체 결과 Scenario 집합은 아래와 같이 확장된다(근거: design.md Decision 1b, 1c, 5, 5a).
-->

### Requirement: Harvester가 실제 HTML을 가져온다

시스템은 Harvester가 크롤 그래프의 노드 URL에서 실제 HTML을 가져올 수 있어야 한다(SHALL). 본 Requirement 블록의 Scenario 집합은 기존 `bot` capability의 동명 Requirement가 보유하던 Scenario 2건을 다음과 같이 대체한다(근거: design.md Decision 1b, 1c). (a) 기존 "Harvester HTML 가져오기" Scenario는 본 블록의 "스냅샷 hit 시 네트워크 호출 없이 로컬 파싱" 및 "출처 무관한 응답 의미론" Scenario로 대체되며, (b) 기존 "Pioneer와 Harvester의 fetch 로직 공유" Scenario는 본 블록의 "Pioneer와 Harvester의 HTTP 경계 설정 공유" Scenario로 **완화 대체**된다(공유 범위가 "동일 fetch 함수"에서 "HTTP 경계 설정 helper"로 축소됨). Harvester는 단일 fetch 진입점에 의존하며, 해당 진입점은 **ObjectStorage 스냅샷을 우선 시도하고 사용 불가 시 HTTP fetch로 폴백**하는 합성 의미론을 제공해야 한다(SHALL). 진입점이 반환하는 바이트열은 출처(스냅샷/HTTP)와 무관하게 동일한 의미의 원본 HTML로 후속 파싱 파이프라인에 전달되어야 한다(SHALL). 참고(informative) — 설계 의사코드: `apps/api/fuguebot_pseudo.go` 라인 97–112. 의사코드의 구체 타입 이름은 구현 세부이며 본 Requirement의 행위 계약 대상이 아니다.

ObjectStorage 조회 시 사용하는 스냅샷 키 포맷과 해시 함수는 본 Requirement에서 자체 정의하지 않고, **동일 `bot` capability 내부의 스냅샷 쓰기 경로(구 `pioneer-snapshot-storage` change에서 확정, 현재는 `bot` capability의 스냅샷 쓰기 Requirement로 존재)가 확정한 공용 키 규약을 그대로 따른다**(SHALL). Harvester 측에서 키 포맷·해시 함수를 재구현해서는 안 된다(MUST NOT).

#### Scenario: 스냅샷 hit 시 네트워크 호출 없이 로컬 파싱
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청하고 ObjectStorage에서 유효한 스냅샷이 반환될 때
- **THEN** Harvester는 외부 사이트로 HTTP 요청을 보내지 않고, ObjectStorage에서 받은 본문만으로 파싱 파이프라인을 진행한다

#### Scenario: 출처 무관한 응답 의미론
- **WHEN** Harvester가 ObjectStorage 또는 HTTP 어느 쪽에서든 본문을 받을 때
- **THEN** 후속 파서/스크립트 실행기는 출처를 구분하지 않고 동일한 의미의 원본 HTML 바이트열을 관찰한다. 저장 경로에 적용된 모든 저장 포맷 변환은 fetch 경계 안에서 완결되어야 하며, 호출자에게 저장 포맷의 세부(예: 압축 등)가 노출되지 않는다

#### Scenario: Pioneer와 Harvester의 HTTP 경계 설정 공유
- **WHEN** Pioneer가 원본을 fetch하거나 Harvester가 스냅샷 사용 불가로 HTTP 폴백 경로로 HTML을 가져올 때(즉 어느 쪽이든 HTTP를 실제로 호출하는 시점)
- **THEN** 동일한 HTTP 경계 설정(사이즈 제한, 리다이렉트 제한, 타임아웃, User-Agent)을 공유하여 중복 구현과 드리프트를 방지한다. 상위 fetch 인터페이스 시그니처까지 동일해야 하는 것은 아니며, HTTP helper 수준의 공유만을 요구한다

#### Scenario: 스냅샷 키 규약은 동일 capability의 쓰기 경로를 참조한다
- **WHEN** Harvester의 ObjectStorage 조회가 스냅샷 키를 계산할 때
- **THEN** 동일 `bot` capability 내부의 스냅샷 쓰기 Requirement가 확정한 공용 키 빌더(normalized URL 해시 기반)를 그대로 사용하며, 본 Requirement에서 키 포맷을 별도로 정의하지 않는다. 또한 키 빌더 입력으로 전달하는 normalized URL은 쓰기 경로가 사용한 URL 정규화 규칙과 동일한 규칙의 결과여야 한다(읽기 키와 쓰기 키가 비트 단위로 일치하도록 보장한다)

#### Scenario: 스냅샷 조회 시간 기준
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청할 때
- **THEN** 스냅샷 키의 시간 세그먼트는 Harvester가 해당 fetch 요청을 수행하는 시점(호출 단위로 관찰되는 현재 UTC 날짜)로 결정되며, 스냅샷 쓰기 경로가 같은 UTC 일자 내에 쓴 스냅샷만 hit 대상이 된다. 그 외(과거 일자에 쓰인 스냅샷 등)는 "사용 불가"로 간주되어 HTTP 폴백 경로로 수렴한다

---

## ADDED Requirements

### Requirement: 스냅샷 사용 불가 시 HTTP fetch로 폴백한다

ObjectStorage 조회가 성공하지 못하는 모든 경우(키 없음, 네트워크/권한/내부 에러 등 일체 — TTL 만료는 lifecycle 삭제에 의해 "키 없음"으로 수렴하며 독립 관측 범주가 아니다)를 Harvester는 단일 "사용 불가(miss)"로 취급하여 동일 노드 URL에 대해 HTTP fetch로 폴백해야 한다(SHALL). 실패 유형에 따라 fetch 동작이 달라져서는 안 된다(MUST NOT). 폴백된 HTTP 응답을 ObjectStorage에 재저장할지 여부는 본 요구사항이 정의하지 않으며, 저장 책임은 Pioneer 쓰기 경로에 위임한다.

ObjectStorage 실패 유형 구분은 **로그/메트릭 레벨에서만** 수행되어야 하며(운영 관찰·알람 임계치 산정용), fetch 의사결정(폴백 여부)에는 영향을 주지 않는다(SHALL). 이는 동작(behavior)이 아니라 관측(observability)의 영역이다.

#### Scenario: 스냅샷 miss 시 HTTP 폴백
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했으나 ObjectStorage에 해당 스냅샷이 존재하지 않을 때(신규 미저장, lifecycle rule에 의한 TTL 만료 삭제, 과거 UTC 일자 키 어긋남 등이 모두 이 단일 케이스로 수렴)
- **THEN** 동일 URL에 대해 HTTP fetch를 수행하여 본문을 획득한 뒤 파싱 파이프라인을 진행한다

#### Scenario: ObjectStorage 에러 시 HTTP 폴백
- **WHEN** Harvester의 ObjectStorage 조회가 네트워크/권한/내부 에러로 실패할 때
- **THEN** Harvester는 즉시 실패로 처리하지 않고 동일 URL에 대해 HTTP fetch로 폴백한다

#### Scenario: 실패 유형은 로그로만 구분된다
- **WHEN** ObjectStorage 조회가 실패할 때
- **THEN** 운영자가 실패 원인을 로그·메트릭을 통해 판별할 수 있도록 관측 데이터가 남지만, fetch 동작은 모든 케이스에서 동일하게 HTTP fallback으로 수렴한다(실패 유형이 fetch 분기에 영향을 주지 않는다). 관측 라벨의 구체적 문자열 집합은 본 spec의 행위 계약 대상이 아니며 운영 설정에서 관리한다

---

### Requirement: ObjectStorage와 HTTP 모두 실패 시 노드 처리 실패로 집계한다

ObjectStorage 경로와 HTTP 폴백 경로가 모두 본문을 반환하지 못하면, Harvester는 해당 노드 처리를 실패로 분류하고 **Harvester 워커의 실행 통계(in-memory, scheduler의 DB 컬럼과 구분되는 별개 집계)** 의 fetch 실패 카운터가 1 증가하도록 해야 한다(SHALL). 이 실패는 다른 노드의 처리를 중단시키지 않아야 하며(SHALL), 단일 노드 실패가 Harvester 실행 전체를 중단시켜서는 안 된다(MUST NOT). 집계 카운터의 내부 식별자 이름은 본 spec의 행위 계약 대상이 아니며 구현 문서에서 정의한다. `harvester_frontier.harvest_error_count` DB 컬럼 증가는 `harvester-scheduler-consumer` capability의 `RecordHarvestError` 경로가 담당하며, 본 Requirement는 해당 DB 경로에 추가 증가를 요구하지 않는다.

#### Scenario: 이중 실패 시 실행 통계 카운터 증가
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했고 ObjectStorage 조회와 HTTP 폴백이 모두 본문을 반환하지 못할 때
- **THEN** 해당 노드의 파싱은 수행되지 않고 Harvester 워커 실행 통계의 fetch 실패 카운터가 정확히 1 증가한다

#### Scenario: 노드 단위 실패 격리
- **WHEN** 특정 노드의 fetch가 이중 실패로 종료될 때
- **THEN** Harvester는 다음 노드의 처리를 계속 진행하며 단일 노드의 실패가 전체 실행을 중단시키지 않는다

#### Scenario: fetch 출처 및 실패 종류의 관측성
- **WHEN** Harvester가 fetch를 수행할 때
- **THEN** 각 호출의 fetch 출처(스냅샷/HTTP) 및 실패 시 ObjectStorage 에러 종류가 로그/메트릭으로 식별 가능하다(운영 관찰용이며 fetch 행위에는 영향을 주지 않는다)
