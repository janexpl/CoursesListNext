-- Unieważnienie i duplikat zaświadczenia.
--
--   - revoked_at, revoke_reason, revoked_by_user_id - POST /certificates/{id}/revoke.
--     Unieważniony dokument zostaje w bazie i na listach; jego numer rejestru jest zajęty
--     na zawsze - także po DELETE /certificates/{id}, które zostaje bez zmian (zapytania
--     rejestru w API liczą unieważnione dokumenty niezależnie od deleted_at).
--   - supersedes_id, duplicate_reason - POST /certificates/{id}/duplicate. Nowy dokument
--     wskazuje oryginał; oryginał "wie", że został zastąpiony, przez odwrotne wyszukanie
--     (indeks unikalny: dokument może mieć tylko jeden nieusunięty duplikat).
--   - CHECK-i pilnują spójności: powód jest niepusty i występuje razem z datą/wskazaniem.
--
-- Wpływ na istniejące dane: brak. Nowe kolumny są NULL dla wszystkich istniejących
-- zaświadczeń (nieunieważnione, niezastąpione); ADD COLUMN bez wartości domyślnej
-- nie przepisuje tabeli.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

ALTER TABLE certificates
    ADD COLUMN revoked_at timestamptz,
    ADD COLUMN revoke_reason text,
    ADD COLUMN revoked_by_user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN supersedes_id bigint REFERENCES certificates(id) ON DELETE RESTRICT,
    ADD COLUMN duplicate_reason text,
    ADD CONSTRAINT certificates_revoke_consistency CHECK (
        (revoked_at IS NULL AND revoke_reason IS NULL)
        OR (revoked_at IS NOT NULL AND revoke_reason IS NOT NULL AND btrim(revoke_reason) <> '')
    ),
    ADD CONSTRAINT certificates_duplicate_consistency CHECK (
        (supersedes_id IS NULL AND duplicate_reason IS NULL)
        OR (supersedes_id IS NOT NULL AND duplicate_reason IS NOT NULL AND btrim(duplicate_reason) <> '')
    ),
    ADD CONSTRAINT certificates_supersedes_not_self CHECK (supersedes_id <> id);

-- Usunięty (DELETE) duplikat nie blokuje wystawienia kolejnego.
CREATE UNIQUE INDEX certificates_supersedes_id_uidx
    ON certificates (supersedes_id)
    WHERE supersedes_id IS NOT NULL AND deleted_at IS NULL;
