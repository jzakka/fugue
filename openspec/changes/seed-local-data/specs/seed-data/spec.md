## ADDED Requirements

### Requirement: 시드 SQL 파일이 존재한다
`apps/api/db/seed.sql` 파일이 존재하며, creators, auth_accounts, works 테이블에 샘플 데이터를 INSERT한다.

#### Scenario: 시드 파일 실행
- **WHEN** 마이그레이션 완료된 DB에 `psql -f db/seed.sql` 실행
- **THEN** creators 5명, auth_accounts 5건, works 10건 이상이 INSERT됨

### Requirement: 시드 데이터는 멱등하다
시드 SQL은 여러 번 실행해도 동일한 결과를 보장한다. TRUNCATE CASCADE로 기존 데이터를 정리한 후 INSERT한다.

#### Scenario: 시드 반복 실행
- **WHEN** `make seed`를 연속 2번 실행
- **THEN** 데이터 중복 없이 동일한 결과 (같은 row 수)

### Requirement: 시드 데이터가 다양한 분야를 포함한다
음악, 영상편집, 미술, 프로그래밍, 시나리오 라이터 5개 분야가 모두 포함되어 추천 로직 테스트가 가능하다.

#### Scenario: 분야별 작품 존재
- **WHEN** 시드 후 `SELECT DISTINCT field FROM works` 조회
- **THEN** 음악, 영상편집, 미술, 프로그래밍, 시나리오 라이터 5개 분야가 모두 존재

### Requirement: Makefile seed 타겟
`apps/api/Makefile`에 `seed` 타겟이 있어 `make seed`로 시드를 실행할 수 있다.

#### Scenario: make seed 실행
- **WHEN** `apps/api/`에서 `make seed` 실행
- **THEN** DB에 시드 데이터가 투입됨

### Requirement: 프로덕션 실행 방지 경고
시드 SQL 파일 상단에 로컬 전용임을 알리는 경고 주석이 포함된다.

#### Scenario: 시드 파일 확인
- **WHEN** `db/seed.sql` 파일의 첫 줄을 확인
- **THEN** 로컬 개발 전용이라는 경고 주석이 존재
