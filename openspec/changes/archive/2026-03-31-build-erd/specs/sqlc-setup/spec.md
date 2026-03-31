## ADDED Requirements

### Requirement: sqlc 설정 파일
시스템은 apps/api/sqlc.yaml 설정 파일을 제공해야 한다(SHALL). PostgreSQL 엔진, 쿼리 경로(db/queries/), 생성 코드 경로(internal/db/)를 설정한다.

#### Scenario: sqlc generate 실행
- **WHEN** `apps/api/` 디렉토리에서 `sqlc generate` 실행 시
- **THEN** `internal/db/` 디렉토리에 Go 코드가 생성된다 (models.go, db.go, querier 파일들)

---

### Requirement: creators CRUD 쿼리
시스템은 creators 테이블의 기본 CRUD 쿼리를 제공해야 한다(SHALL). 파일: db/queries/creators.sql.

#### Scenario: 크리에이터 생성
- **WHEN** CreateCreator 쿼리 실행 시
- **THEN** nickname, roles, contacts를 받아 새 creators 레코드를 생성하고 결과를 반환한다

#### Scenario: 크리에이터 조회
- **WHEN** GetCreator 쿼리에 id를 전달하면
- **THEN** 해당 크리에이터의 전체 프로필을 반환한다

#### Scenario: 크리에이터 프로필 수정
- **WHEN** UpdateCreator 쿼리 실행 시
- **THEN** nickname, bio, roles, contacts, avatar_url을 업데이트하고 updated_at을 갱신한다

#### Scenario: 역할 태그로 크리에이터 검색
- **WHEN** ListCreatorsByRoles 쿼리에 역할 배열을 전달하면
- **THEN** 해당 역할을 가진 크리에이터 목록을 페이지네이션하여 반환한다

---

### Requirement: auth_accounts CRUD 쿼리
시스템은 auth_accounts 테이블의 쿼리를 제공해야 한다(SHALL). 파일: db/queries/auth_accounts.sql.

#### Scenario: provider로 계정 조회
- **WHEN** GetAuthAccountByProvider 쿼리에 provider, provider_id를 전달하면
- **THEN** 해당 OAuth 계정과 연결된 creator_id를 반환한다

#### Scenario: 이메일로 계정 조회
- **WHEN** GetAuthAccountByEmail 쿼리에 email을 전달하면
- **THEN** 해당 이메일을 가진 auth_account 목록을 반환한다

#### Scenario: 계정 생성
- **WHEN** CreateAuthAccount 쿼리 실행 시
- **THEN** creator_id, provider, provider_id, email을 받아 새 레코드를 생성한다

---

### Requirement: works CRUD 쿼리
시스템은 works 테이블의 CRUD 쿼리를 제공해야 한다(SHALL). 파일: db/queries/works.sql.

#### Scenario: 작품 생성
- **WHEN** CreateWork 쿼리 실행 시
- **THEN** creator_id, url, title, description, field, tags, og_image, og_data를 받아 새 레코드를 생성한다

#### Scenario: 작품 조회
- **WHEN** GetWork 쿼리에 id를 전달하면
- **THEN** 해당 작품의 전체 정보를 반환한다

#### Scenario: 작품 삭제
- **WHEN** DeleteWork 쿼리에 id와 creator_id를 전달하면
- **THEN** 해당 작품이 본인의 작품인 경우에만 삭제한다

#### Scenario: 분야/태그 필터 조회
- **WHEN** ListWorks 쿼리에 field, tags, page, limit를 전달하면
- **THEN** 조건에 맞는 작품 목록을 페이지네이션하여 반환한다

#### Scenario: 태그 매칭 기반 추천 조회
- **WHEN** RecommendWorks 쿼리에 필요 분야 목록과 태그 배열을 전달하면
- **THEN** 해당 분야의 작품을 태그 교집합 크기 내림차순, 최신순으로 반환한다
