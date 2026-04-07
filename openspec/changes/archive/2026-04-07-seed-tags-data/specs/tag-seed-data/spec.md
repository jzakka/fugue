## ADDED Requirements

### Requirement: 크로스미디어 분야별 사전 정의 태그를 제공한다
시스템은 음악, 일러스트, 영상, 글, 코드, 공통 카테고리에 걸쳐 사전 정의된 태그 데이터를 보유해야 한다(SHALL).

#### Scenario: 태그 데이터가 존재한다
- **WHEN** `make dev`로 로컬 환경을 셋업하면
- **THEN** tags 테이블에 카테고리별 태그가 존재하며, `GET /api/tags` 응답에 태그 목록이 포함된다

#### Scenario: 모든 카테고리에 태그가 존재한다
- **WHEN** 전체 태그 목록을 조회하면
- **THEN** 음악, 일러스트, 영상, 글, 코드, 공통 카테고리 각각에 1개 이상의 태그가 존재한다

#### Scenario: 시드 핀에 태그가 연결된다
- **WHEN** 시드 데이터가 로드된 후 시드 핀을 조회하면
- **THEN** 각 핀에 1개 이상의 태그가 연결되어 있다

---

### Requirement: 태그 시드는 멱등하게 실행된다
시드 스크립트를 여러 번 실행해도 동일한 결과를 보장해야 한다(SHALL).

#### Scenario: 반복 실행 시 에러 없음
- **WHEN** `make seed`를 연속 2회 실행하면
- **THEN** 에러 없이 완료되고, tags 테이블의 행 수가 동일하다

---

### Requirement: 시드 실행 순서가 보장된다
Makefile의 seed 파이프라인이 seed_tags.sql → seed.sql 순서로 실행되어야 한다(SHALL).

#### Scenario: seed_tags.sql이 seed.sql보다 먼저 실행된다
- **WHEN** `make seed`를 실행하면
- **THEN** tags 데이터가 먼저 삽입되고, 이후 seed.sql의 pin_tags INSERT가 정상적으로 태그를 참조한다
