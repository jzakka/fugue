## ADDED Requirements

### Requirement: creators 테이블
시스템은 크리에이터 프로필을 저장하는 creators 테이블을 제공해야 한다(SHALL). 컬럼: id (UUID PK), nickname (VARCHAR(50) NOT NULL), bio (VARCHAR(200)), roles (TEXT[] NOT NULL), contacts (JSONB NOT NULL), avatar_url (VARCHAR(500)), created_at (TIMESTAMPTZ), updated_at (TIMESTAMPTZ).

#### Scenario: 크리에이터 생성
- **WHEN** 첫 OAuth 로그인이 완료되면
- **THEN** creators 레코드가 생성되고, id는 UUID 자동 생성, created_at/updated_at는 현재 시각으로 설정된다

#### Scenario: roles GIN 인덱스
- **WHEN** 역할 태그로 크리에이터를 검색하면
- **THEN** GIN 인덱스(idx_creators_roles)를 통해 효율적으로 조회된다

---

### Requirement: auth_accounts 테이블
시스템은 OAuth 인증 계정을 저장하는 auth_accounts 테이블을 제공해야 한다(SHALL). 컬럼: id (UUID PK), creator_id (UUID FK → creators.id NOT NULL), provider (VARCHAR(20) NOT NULL), provider_id (VARCHAR(255) NOT NULL), email (VARCHAR(255)), created_at (TIMESTAMPTZ). UNIQUE 제약: (provider, provider_id).

#### Scenario: 같은 provider로 중복 계정 방지
- **WHEN** 동일한 (provider, provider_id) 조합으로 auth_account 생성을 시도하면
- **THEN** UNIQUE 제약 위반 에러가 발생한다

#### Scenario: 이메일 기반 계정 병합
- **WHEN** 새 OAuth 로그인의 이메일이 기존 auth_account의 이메일과 일치하면
- **THEN** 기존 creator_id에 새 auth_account를 연결하여 계정을 병합한다

#### Scenario: 이메일 없는 provider
- **WHEN** OAuth provider가 이메일을 제공하지 않으면
- **THEN** email 컬럼은 NULL로 저장되고, 이메일 기반 병합은 수행하지 않는다

---

### Requirement: works 테이블
시스템은 투고 작품을 저장하는 works 테이블을 제공해야 한다(SHALL). 컬럼: id (UUID PK), creator_id (UUID FK → creators.id NOT NULL), url (VARCHAR(1000) NOT NULL), title (VARCHAR(200) NOT NULL), description (VARCHAR(500)), field (VARCHAR(50) NOT NULL), tags (TEXT[] NOT NULL), og_image (VARCHAR(1000)), og_data (JSONB), created_at (TIMESTAMPTZ).

#### Scenario: 작품 생성
- **WHEN** 크리에이터가 작품을 투고하면
- **THEN** works 레코드가 생성되고, creator_id는 로그인한 유저의 ID로 설정된다

#### Scenario: 분야별 조회
- **WHEN** 특정 분야(field)로 작품을 필터링하면
- **THEN** idx_works_field 인덱스를 통해 효율적으로 조회된다

#### Scenario: 태그 기반 검색
- **WHEN** 태그 배열로 작품을 검색하면
- **THEN** GIN 인덱스(idx_works_tags)를 통해 배열 교집합 연산이 효율적으로 수행된다

#### Scenario: 크리에이터별 작품 조회
- **WHEN** 특정 크리에이터의 작품 목록을 조회하면
- **THEN** idx_works_creator 인덱스를 통해 효율적으로 조회된다

---

### Requirement: project_types 테이블
시스템은 프로젝트 유형별 필요 분야 매핑을 저장하는 project_types 테이블을 제공해야 한다(SHALL). 컬럼: id (VARCHAR(50) PK), name (VARCHAR(100) NOT NULL), required_fields (TEXT[] NOT NULL). 시드 데이터로 MV, 게임, 앨범 아트워크, 애니메이션, 보이스드라마, 기타를 포함한다.

#### Scenario: 프로젝트 유형으로 추천 필요 분야 조회
- **WHEN** 추천 API에서 프로젝트 유형 ID를 받으면
- **THEN** project_types에서 required_fields를 조회하여 추천 대상 분야 목록을 반환한다

#### Scenario: 시드 데이터 존재
- **WHEN** 마이그레이션이 완료되면
- **THEN** 6개 기본 프로젝트 유형(mv, game, album-artwork, animation, voice-drama, other)이 존재한다

---

### Requirement: 마이그레이션 파일
시스템은 golang-migrate 형식의 마이그레이션 파일을 제공해야 한다(SHALL). 파일 위치: apps/api/db/migrations/. up/down 쌍으로 구성.

#### Scenario: 마이그레이션 적용
- **WHEN** `migrate -path apps/api/db/migrations -database $DB_URL up` 실행 시
- **THEN** 모든 테이블, 인덱스, 제약조건이 생성된다

#### Scenario: 마이그레이션 롤백
- **WHEN** `migrate -path apps/api/db/migrations -database $DB_URL down` 실행 시
- **THEN** 생성된 테이블이 역순으로 삭제된다
