-- docs/erd.md L86 "tag_id | UUID FK → tags | ON DELETE CASCADE" 명시와 정렬.
-- 000011_create_tags.up.sql 에서 tag_id FK 가 cascade rule 없이 생성되어 postgres
-- default(NO ACTION) 로 동작 — admin tag 삭제 시점에 child 존재로 차단되어 ERD 가정
-- (cascade) 과 어긋남.
ALTER TABLE pin_tags
    DROP CONSTRAINT pin_tags_tag_id_fkey,
    ADD CONSTRAINT pin_tags_tag_id_fkey
        FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE;
