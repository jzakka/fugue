# 008 마이그레이션: boards + board_pins 테이블

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규

## 스키마

```sql
CREATE TABLE boards (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id  UUID NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    description VARCHAR(500),
    is_public   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_boards_creator ON boards(creator_id);

CREATE TABLE board_pins (
    board_id    UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    work_id     UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (board_id, work_id)
);
```

## 할 일

- [ ] 마이그레이션 파일 작성 (000008_create_boards.up/down.sql)
- [ ] sqlc 쿼리 작성 (boards.sql, board_pins.sql)
- [ ] sqlc generate 실행
