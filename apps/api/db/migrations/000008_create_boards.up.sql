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
