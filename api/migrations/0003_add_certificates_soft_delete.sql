ALTER TABLE certificates
    ADD COLUMN deleted_at timestamptz,
    ADD COLUMN deleted_by_user_id bigint REFERENCES users(id) ON DELETE RESTRICT,
     ADD COLUMN delete_reason text;

CREATE INDEX certificates_deleted_at_idx ON certificates (deleted_at);
