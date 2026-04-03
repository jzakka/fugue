# 009 마이그레이션: interactions 테이블

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규

## 스키마

```sql
CREATE TABLE interactions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
    work_id     UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    type        VARCHAR(20) NOT NULL,  -- 'view', 'pin', 'board_add'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_interactions_user_time ON interactions(user_id, created_at DESC);
CREATE INDEX idx_interactions_work ON interactions(work_id);
CREATE INDEX idx_interactions_type ON interactions(type);
```

## 설계 메모

- 쓰기 많은 테이블 (모든 페이지뷰마다 INSERT)
- MVP에서는 파티셔닝 불필요 (~1,000 INSERT/day)
- 추후 ML 학습 데이터로 활용 예정 → 데이터 보존 무제한

## 할 일

- [ ] 마이그레이션 파일 작성 (000009_create_interactions.up/down.sql)
- [ ] sqlc 쿼리 작성 (interactions.sql)
- [ ] sqlc generate 실행
