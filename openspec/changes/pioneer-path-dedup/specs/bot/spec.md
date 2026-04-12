## MODIFIED Requirements

### Requirement: 그래프 노드와 엣지를 관리한다
시스템은 크롤링한 페이지를 노드로, 링크를 엣지로 저장하며 중복을 방지해야 한다. 노드는 개별 URL이 아니라 **페이지 템플릿 패턴**을 표현해야 한다(SHALL).

#### Scenario: 쿼리 파라미터가 다른 URL을 동일 노드로 합침
- **WHEN** Pioneer가 `aaa/bbb?x=1`과 `aaa/bbb?x=2`를 발견할 때
- **THEN** 시스템은 두 URL을 동일한 노드로 처리한다

#### Scenario: 숫자 ID가 다른 URL을 동일 노드로 합침
- **WHEN** Pioneer가 `/artworks/12345`와 `/artworks/67890`을 발견할 때
- **THEN** 시스템은 두 URL을 동일한 노드로 처리한다 (path 내 숫자 전용 세그먼트는 동일 패턴으로 간주)

#### Scenario: 다중 숫자 세그먼트 치환
- **WHEN** Pioneer가 `/user/123/post/456`과 `/user/789/post/012`를 발견할 때
- **THEN** 시스템은 두 URL을 동일한 노드로 처리한다

#### Scenario: 비숫자 slug는 보존
- **WHEN** Pioneer가 `/contest/magicalparty`를 발견할 때
- **THEN** 시스템은 고유한 노드로 생성한다 (숫자가 아닌 세그먼트는 구분됨)

#### Scenario: 혼합 문자열 세그먼트는 보존
- **WHEN** Pioneer가 `/item/abc123`을 발견할 때
- **THEN** 시스템은 고유한 노드로 생성한다 (순수 숫자가 아닌 세그먼트는 구분됨)

#### Scenario: 원본 URL 보존
- **WHEN** 새 패턴의 첫 번째 URL이 발견될 때
- **THEN** 시스템은 해당 원본 URL을 보존하여 이후 실제 페이지 접근에 사용할 수 있게 한다

#### Scenario: 이미 존재하는 패턴의 URL 발견
- **WHEN** 동일 패턴에 해당하는 URL이 이미 노드로 존재할 때
- **THEN** 시스템은 새 노드를 생성하지 않고 기존 노드를 재사용한다

#### Scenario: Harvester가 원본 URL로 페이지를 접근
- **WHEN** Harvester가 노드를 처리할 때
- **THEN** 시스템은 canonical path가 아닌 보존된 원본 URL을 사용하여 실제 페이지를 fetch한다

#### Scenario: 중복 엣지 방지
- **WHEN** 같은 링크를 여러 번 발견했을 때
- **THEN** 시스템은 하나의 엣지만 유지한다

#### Scenario: 사이트의 모든 listing 페이지 조회
- **WHEN** 특정 사이트의 listing 타입 노드들을 조회할 때
- **THEN** 시스템은 해당하는 모든 노드를 빠르게 반환한다
