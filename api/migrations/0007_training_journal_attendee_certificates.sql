ALTER TABLE training_journal_attendees
    ADD COLUMN certificate_id bigint REFERENCES certificates(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX training_journal_attendees_certificate_id_uidx
    ON training_journal_attendees (certificate_id)
    WHERE certificate_id IS NOT NULL;
