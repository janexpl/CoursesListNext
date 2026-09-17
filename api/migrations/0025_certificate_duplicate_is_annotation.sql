-- Duplikat zaświadczenia to ten sam dokument, nie nowy.
--
-- Wtórnik nosi ten sam numer rejestru co oryginał i adnotację "DUPLIKAT" z datą
-- wystawienia. Migracja 0023 wprowadzała duplikat jako osobny dokument (supersedes_id
-- i własny numer rejestru), co oznaczałoby, że kursant ma dwa różne zaświadczenia
-- z jednego szkolenia - wycofujemy tamten model.
--
--   - duplicate_issued_at          - data wystawienia duplikatu (drukowana na dokumencie),
--   - duplicate_issued_by_user_id  - kto go wystawił,
--   - duplicate_reason             - powód (kolumna z 0023, teraz opisuje ten sam dokument).
--
-- Wpływ na istniejące dane: jeśli w bazie są dokumenty wystawione jako osobne duplikaty
-- (niepuste supersedes_id), migracja przerywa się - scalenie dwóch dokumentów w jeden
-- jest decyzją merytoryczną i nie może dziać się automatycznie. Na produkcji migracje
-- 0023-0024 nie były jeszcze uruchamiane, więc warunek nie wystąpi.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

DO $$
DECLARE
    separate_duplicates integer;
BEGIN
    SELECT count(*) INTO separate_duplicates FROM certificates WHERE supersedes_id IS NOT NULL;

    IF separate_duplicates > 0 THEN
        RAISE EXCEPTION
            'w bazie jest % zaświadczeń wystawionych jako osobne duplikaty - wycofaj je przed migracją (SELECT id, supersedes_id FROM certificates WHERE supersedes_id IS NOT NULL)',
            separate_duplicates;
    END IF;
END $$;

ALTER TABLE certificates
    DROP CONSTRAINT IF EXISTS certificates_duplicate_consistency,
    DROP CONSTRAINT IF EXISTS certificates_supersedes_not_self,
    DROP COLUMN IF EXISTS supersedes_id,
    ADD COLUMN duplicate_issued_at timestamptz,
    ADD COLUMN duplicate_issued_by_user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT certificates_duplicate_consistency CHECK (
        (duplicate_issued_at IS NULL AND duplicate_reason IS NULL)
        OR (duplicate_issued_at IS NOT NULL AND duplicate_reason IS NOT NULL AND btrim(duplicate_reason) <> '')
    );
