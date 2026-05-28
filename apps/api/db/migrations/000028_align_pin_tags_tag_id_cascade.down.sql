-- 역연산: cascade rule 제거 후 default(NO ACTION) 로 복귀.
ALTER TABLE pin_tags
    DROP CONSTRAINT pin_tags_tag_id_fkey,
    ADD CONSTRAINT pin_tags_tag_id_fkey
        FOREIGN KEY (tag_id) REFERENCES tags(id);
