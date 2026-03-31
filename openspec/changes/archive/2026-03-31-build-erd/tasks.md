## 1. 프로젝트 구조 셋업

- [x] 1.1 apps/api/ 디렉토리 구조 생성 (cmd/server/, internal/db/, db/migrations/, db/queries/)
- [x] 1.2 Go module 초기화 (go mod init)
- [x] 1.3 golang-migrate CLI 설치 및 Makefile에 migrate 명령 추가

## 2. DDL 마이그레이션 작성

- [x] 2.1 001_create_creators.up.sql / down.sql 작성 (creators 테이블 + GIN 인덱스)
- [x] 2.2 002_create_auth_accounts.up.sql / down.sql 작성 (auth_accounts 테이블 + UNIQUE 제약 + 인덱스)
- [x] 2.3 003_create_works.up.sql / down.sql 작성 (works 테이블 + field, tags GIN, creator_id 인덱스)
- [x] 2.4 004_create_project_types.up.sql / down.sql 작성 (project_types 테이블 + 시드 데이터 6건)
- [x] 2.5 docker-compose.yml의 PostgreSQL로 마이그레이션 실행 확인

## 3. sqlc 설정

- [x] 3.1 apps/api/sqlc.yaml 작성 (PostgreSQL 엔진, 쿼리/생성 경로 설정)
- [x] 3.2 db/queries/creators.sql 작성 (CreateCreator, GetCreator, UpdateCreator, ListCreatorsByRoles)
- [x] 3.3 db/queries/auth_accounts.sql 작성 (CreateAuthAccount, GetAuthAccountByProvider, GetAuthAccountByEmail)
- [x] 3.4 db/queries/works.sql 작성 (CreateWork, GetWork, DeleteWork, ListWorks, RecommendWorks)
- [x] 3.5 sqlc generate 실행 및 생성된 Go 코드 확인

## 4. 검증

- [x] 4.1 docker-compose up으로 PostgreSQL 기동 후 전체 마이그레이션 up/down 왕복 테스트
- [x] 4.2 생성된 sqlc 코드가 컴파일되는지 go build 확인
- [x] 4.3 docs/erd.md에 확정된 ERD 작성 (문서 구조 변경으로 CLAUDE.md 대신 docs/erd.md)
