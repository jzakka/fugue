## Context

Fugue MVP는 creators, works 2개 테이블 초안만 존재한다. OAuth 계정 병합을 위한 auth_accounts, 프로젝트 유형 매핑을 위한 project_types 등 누락된 테이블이 있고, sqlc 설정도 아직 없다. DB 스키마를 확정해야 API 구현이 시작될 수 있다.

기술 스택: PostgreSQL 16, Go + Chi, sqlc, Docker Compose (로컬), CloudNativePG (dev), RDS (prod).

## Goals / Non-Goals

**Goals:**
- MVP 전체 ERD 확정 (인증, 프로필, 작품, 추천에 필요한 모든 테이블)
- PostgreSQL DDL 마이그레이션 파일 작성
- sqlc 설정 + 기본 CRUD 쿼리 파일 구성
- 계정 병합을 지원하는 auth_accounts 테이블 설계

**Non-Goals:**
- ML 기반 추천 시스템용 테이블 (v2)
- 메시징/채팅 테이블 (v2)
- 알림 시스템 테이블 (v2)
- 결제/구독 테이블

## Decisions

### 1. auth_accounts 분리 테이블로 계정 병합 지원

creators 테이블에 provider 컬럼을 넣는 대신, 별도 auth_accounts 테이블로 1:N 관계를 만든다.

- **선택**: auth_accounts 분리 테이블
- **대안**: creators에 provider/provider_id 컬럼 추가
- **이유**: 한 유저가 Google + Discord + Twitter 3개 모두로 로그인 가능해야 한다. 컬럼 방식은 provider 수만큼 컬럼이 늘어나고 병합 로직이 복잡해진다.

### 2. ERD 구조

```
auth_accounts ──┐
                ├──→ creators ←── works
                │
project_types ──┘ (독립, 추천 시 JOIN)
```

**테이블 목록:**
- `creators`: 크리에이터 프로필 (닉네임, 역할, 소개, 연락처, 아바타)
- `auth_accounts`: OAuth 인증 계정 (provider, provider_id, email). creators와 N:1
- `works`: 투고 작품 (URL, 제목, 분야, 태그, OG 데이터). creators와 N:1
- `project_types`: 프로젝트 유형별 필요 분야 매핑 (MV→일러스트,영상 등). 시드 데이터

### 3. 마이그레이션 도구: golang-migrate

- **선택**: golang-migrate
- **대안**: goose, Atlas, 수동 SQL
- **이유**: Go 생태계 표준. 버전 넘버링 방식으로 순서 보장. CLI + 라이브러리 모두 지원. docker-compose에서 자동 마이그레이션 가능.

### 4. sqlc 설정

- `sqlc.yaml`을 `apps/api/` 루트에 배치
- 쿼리 파일: `apps/api/db/queries/*.sql`
- 생성 코드: `apps/api/internal/db/`
- PostgreSQL 엔진 사용

### 5. UUID vs BIGSERIAL

- **선택**: UUID (gen_random_uuid())
- **대안**: BIGSERIAL
- **이유**: API에서 ID 노출 시 순차 ID는 리소스 열거 공격에 취약. UUID는 URL에 바로 사용 가능. PostgreSQL 16의 gen_random_uuid()는 extension 없이 사용 가능.

## Risks / Trade-offs

- **UUID 성능**: B-tree 인덱스에서 BIGSERIAL보다 느리다 → MVP 규모에서는 무시 가능. 수백만 row 이전에는 문제 없음
- **project_types 하드코딩 vs DB**: DB에 넣으면 관리 유연하지만 캐싱 필요 → 시드 데이터로 넣고 Redis 캐싱 또는 앱 시작 시 메모리 로드
- **마이그레이션 충돌**: 1인 개발이므로 리스크 낮음
- **auth_accounts 이메일 병합**: provider마다 이메일 제공 여부가 다름 (Twitter는 별도 스코프 필요) → 이메일 없는 경우 병합 불가, 별도 계정으로 생성
