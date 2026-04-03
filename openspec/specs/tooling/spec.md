## ADDED Requirements

### Requirement: DB 마이그레이션을 관리한다
시스템은 golang-migrate 형식의 마이그레이션 파일로 스키마를 관리해야 한다(SHALL).

#### Scenario: 마이그레이션 적용
- **WHEN** 마이그레이션 up 명령을 실행하면
- **THEN** 모든 테이블, 인덱스, 제약조건이 생성된다

#### Scenario: 마이그레이션 롤백
- **WHEN** 마이그레이션 down 명령을 실행하면
- **THEN** 테이블이 역순으로 삭제된다

---

### Requirement: sqlc로 타입 안전한 쿼리 코드를 생성한다
시스템은 SQL 쿼리 파일로부터 Go 코드를 자동 생성해야 한다(SHALL).

#### Scenario: 쿼리 코드 생성
- **WHEN** sqlc generate를 실행하면
- **THEN** SQL 쿼리에 대응하는 타입 안전한 Go 함수가 생성된다

---

### Requirement: 커밋 시 자동 린트를 실행한다
시스템은 커밋 시 pre-commit hook으로 린트와 포맷 검사를 자동 실행해야 한다(SHALL).

#### Scenario: 린트 통과
- **WHEN** 린트 위반이 없는 코드를 커밋하면
- **THEN** 커밋이 성공한다

#### Scenario: 린트 실패
- **WHEN** 린트 위반이 있는 코드를 커밋하면
- **THEN** 커밋이 차단된다

#### Scenario: 자동 포맷
- **WHEN** 포맷되지 않은 Go 코드를 커밋하면
- **THEN** 자동으로 포맷된 후 커밋에 포함된다
