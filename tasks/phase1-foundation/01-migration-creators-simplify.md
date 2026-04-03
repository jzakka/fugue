# 007 마이그레이션: creators 테이블 간소화

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 변경 (기존 코드 수정)

## 배경

큐레이션 모델에서 프로필은 단순 계정 (닉네임 + 아바타). roles, bio, contacts 필드가 불필요해짐.

## 현재 상태

```sql
-- 000001_create_creators.up.sql
CREATE TABLE creators (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nickname    VARCHAR(50) NOT NULL,
    bio         VARCHAR(200),            -- 불필요
    roles       TEXT[] NOT NULL,          -- 불필요, NOT NULL 제약
    contacts    JSONB NOT NULL,           -- 불필요, NOT NULL 제약
    avatar_url  VARCHAR(500),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 결정 필요

- **Option A**: 컬럼 DROP (깔끔하지만 되돌리기 어려움)
- **Option B**: nullable로 변경 (안전하지만 죽은 컬럼 남음)

## 할 일

- [ ] 마이그레이션 파일 작성 (000007_simplify_creators.up/down.sql)
- [ ] roles/contacts의 NOT NULL 제약 또는 컬럼 자체 제거
- [ ] idx_creators_roles 인덱스 제거
- [ ] sqlc 쿼리 수정 (CreateCreatorFromOAuth, UpdateCreator 등)
- [ ] sqlc generate 실행
- [ ] Go 코드 수정 (creator handler, dto, auth service)

## 영향 범위

- `apps/api/db/migrations/`
- `apps/api/db/queries/creators.sql`
- `apps/api/internal/creator/handler.go`
- `apps/api/internal/creator/dto.go`
- `apps/api/internal/auth/service.go` (OAuth 가입 시 roles/contacts 전달 부분)
