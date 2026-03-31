## Why

CLAUDE.md에 creators/works 2개 테이블의 초안 스키마가 있지만, OAuth 계정 병합(auth_accounts), 프로젝트 유형 매핑 등 MVP 전체를 커버하는 ERD가 없다. DB 스키마를 확정해야 sqlc 코드 생성, API 구현, 프론트엔드 타입 정의가 시작될 수 있다.

## What Changes

- MVP 전체 ERD 설계 (creators, works, auth_accounts, project_types 등)
- PostgreSQL DDL 마이그레이션 파일 작성
- sqlc 쿼리 파일 및 설정 작성
- 기존 CLAUDE.md의 초안 스키마를 확정된 ERD로 갱신

## Capabilities

### New Capabilities
- `database-schema`: MVP 전체 ERD 설계. creators, works, auth_accounts 테이블 정의, 인덱스, 제약조건
- `sqlc-setup`: sqlc 설정 및 기본 CRUD 쿼리 파일 구성

### Modified Capabilities

## Impact

- `apps/api/`: sqlc 설정 및 생성 코드 디렉토리 추가
- DB 마이그레이션 파일 추가 (apps/api/db/migrations/)
- CLAUDE.md의 DB 스키마 섹션 업데이트
- 이후 모든 API 구현이 이 스키마에 의존
