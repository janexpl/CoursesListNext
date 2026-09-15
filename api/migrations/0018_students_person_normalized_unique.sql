-- Blokuje duplikaty osób niezależnie od wielkości liter i spacji na brzegach imienia
-- i nazwiska: "Jan Kowalski" i "JAN KOWALSKI " z tą samą datą urodzenia to ta sama osoba.
--
-- WYMAGA WCZEŚNIEJSZEGO UPORZĄDKOWANIA DUPLIKATÓW. Migracja przerywa się, dopóki w tabeli
-- students istnieją rekordy różniące się tylko wielkością liter lub spacjami. Scalenie
-- kursantów oznacza przepięcie ich zaświadczeń i udziałów w dziennikach na jeden rekord -
-- to decyzja merytoryczna (m.in. który PESEL i która firma są poprawne), więc migracja
-- nie robi tego sama.
--
-- Zapytanie raportowe - lista grup duplikatów do przejrzenia:
--
--   SELECT
--       lower(btrim(s.lastname)) AS nazwisko,
--       lower(btrim(s.firstname)) AS imie,
--       s.birthdate,
--       s.id,
--       '"' || s.firstname || '" "' || s.lastname || '"' AS zapis_dokladny,
--       s.pesel,
--       c.name AS firma,
--       (SELECT count(*) FROM certificates cert
--         WHERE cert.student_id = s.id AND cert.deleted_at IS NULL) AS zaswiadczenia,
--       (SELECT count(*) FROM training_journal_attendees a
--         WHERE a.student_id = s.id) AS udzialy_w_dziennikach
--   FROM students s
--   LEFT JOIN companies c ON c.id = s.company_id
--   WHERE (lower(btrim(s.firstname)), lower(btrim(s.lastname)), s.birthdate) IN (
--       SELECT lower(btrim(firstname)), lower(btrim(lastname)), birthdate
--       FROM students
--       GROUP BY 1, 2, 3
--       HAVING count(*) > 1
--   )
--   ORDER BY 1, 2, 3, s.id;
--
-- Wymaga migracji 0017. Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

DO $$
DECLARE
    duplicate_groups integer;
BEGIN
    SELECT count(*) INTO duplicate_groups
    FROM (
        SELECT 1
        FROM students
        GROUP BY lower(btrim(firstname)), lower(btrim(lastname)), birthdate
        HAVING count(*) > 1
    ) groups;

    IF duplicate_groups > 0 THEN
        RAISE EXCEPTION
            'znaleziono % grup kursantów różniących się tylko wielkością liter lub spacjami - uporządkuj je przed migracją (zapytanie raportowe w nagłówku pliku)',
            duplicate_groups;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS students_person_normalized_uidx
    ON students (lower(btrim(firstname)), lower(btrim(lastname)), birthdate);

-- API szuka duplikatu równością na wszystkich trzech wyrażeniach, więc zapytanie może
-- korzystać z indeksu unikalnego - pomocniczy indeks z 0017 staje się zbędny.
DROP INDEX IF EXISTS students_person_lookup_idx;
