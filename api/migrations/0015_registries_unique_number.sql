-- Porządkuje duplikaty numerów rejestru i wymusza ich unikalność na poziomie bazy:
--   1) soft-delete duplikatów tego samego kursanta (zostaje najstarszy certyfikat),
--   2) renumeracja konfliktów między różnymi kursantami (młodszy certyfikat
--      dostaje kolejny wolny numer w ramach kursu i roku),
--   3) deduplikacja wierszy registries,
--   4) constrainty docelowe.
-- Uruchamiać przez: psql --single-transaction
-- Przed uruchomieniem zapisz listę konfliktów (zapytanie podglądowe w PR/notatkach),
-- żeby wiedzieć, które zaświadczenia wymagają dodruku po renumeracji.

-- 1. Soft-delete duplikatów w obrębie tego samego kursanta.
--    Zostaje certyfikat o najniższym id, pozostałe dostają deleted_at.
WITH same_student_duplicates AS (
    SELECT c.id,
           row_number() OVER (
               PARTITION BY r.course_id, r.year, r.number, c.student_id
               ORDER BY c.id
           ) AS rn
    FROM certificates c
    JOIN registries r ON r.id = c.registry_id
    WHERE c.deleted_at IS NULL
)
UPDATE certificates
SET deleted_at    = now(),
    delete_reason = 'duplikat numeru rejestru dla tego samego kursanta (migracja 0015)'
WHERE id IN (SELECT id FROM same_student_duplicates WHERE rn > 1);

-- 2. Bezpiecznik: dwa aktywne certyfikaty wskazujące na TEN SAM wiersz registries
--    uniemożliwiłyby renumerację (zmiana numeru objęłaby oba) i tak czy inaczej
--    zablokują indeks z kroku 5. Taki przypadek wymaga ręcznej korekty.
DO $$
DECLARE
    shared_count integer;
BEGIN
    SELECT count(*) INTO shared_count
    FROM (
        SELECT registry_id
        FROM certificates
        WHERE deleted_at IS NULL
        GROUP BY registry_id
        HAVING count(*) > 1
    ) shared;

    IF shared_count > 0 THEN
        RAISE EXCEPTION
            'znaleziono % wierszy registries współdzielonych przez aktywne certyfikaty - wymagana ręczna korekta przed migracją',
            shared_count;
    END IF;
END $$;

-- 3. Renumeracja konfliktów między różnymi kursantami.
--    W każdej grupie (course_id, year, number) najstarszy certyfikat zachowuje numer,
--    kolejne dostają max(number)+1, +2, ... w ramach swojego kursu i roku.
--    Maksimum liczone po wszystkich wierszach registries (także po soft-usuniętych
--    certyfikatach), żeby nowy numer nie kolidował z niczym istniejącym.
WITH conflicts AS (
    SELECT c.id AS cert_id,
           c.registry_id,
           r.course_id,
           r.year,
           row_number() OVER (
               PARTITION BY r.course_id, r.year, r.number
               ORDER BY c.id
           ) AS rn_in_group
    FROM certificates c
    JOIN registries r ON r.id = c.registry_id
    WHERE c.deleted_at IS NULL
),
to_renumber AS (
    SELECT registry_id, course_id, year,
           row_number() OVER (
               PARTITION BY course_id, year
               ORDER BY cert_id
           ) AS seq
    FROM conflicts
    WHERE rn_in_group > 1
),
max_numbers AS (
    SELECT course_id, year, max(number) AS max_number
    FROM registries
    GROUP BY course_id, year
)
UPDATE registries r
SET number = m.max_number + t.seq
FROM to_renumber t
JOIN max_numbers m
  ON m.course_id = t.course_id
 AND m.year = t.year
WHERE r.id = t.registry_id;

-- 4. Deduplikacja registries: przepnij wszystkie certyfikaty (także soft-usunięte)
--    na kanoniczny wiersz grupy (min(id)), potem usuń niekanoniczne wiersze.
--    Kolejność jest krytyczna - certificates.registry_id ma ON DELETE CASCADE.
WITH canonical AS (
    SELECT min(id) AS keep_id, course_id, year, number
    FROM registries
    GROUP BY course_id, year, number
    HAVING count(*) > 1
)
UPDATE certificates cert
SET registry_id = canonical.keep_id
FROM registries r
JOIN canonical
  ON canonical.course_id = r.course_id
 AND canonical.year = r.year
 AND canonical.number = r.number
WHERE cert.registry_id = r.id
  AND r.id <> canonical.keep_id;

-- Warunek NOT EXISTS to bezpiecznik przed CASCADE - gdyby coś nadal wskazywało
-- na niekanoniczny wiersz, zostanie on pominięty, a krok 5 głośno zawiedzie.
DELETE FROM registries r
USING (
    SELECT min(id) AS keep_id, course_id, year, number
    FROM registries
    GROUP BY course_id, year, number
    HAVING count(*) > 1
) d
WHERE r.course_id = d.course_id
  AND r.year = d.year
  AND r.number = d.number
  AND r.id <> d.keep_id
  AND NOT EXISTS (
      SELECT 1 FROM certificates c WHERE c.registry_id = r.id
  );

-- 5. Constrainty docelowe:
--    jeden wiersz registries na (course_id, year, number)
--    + co najwyżej jeden aktywny certyfikat na wiersz rejestru.
ALTER TABLE registries
    ADD CONSTRAINT registries_course_year_number_key UNIQUE (course_id, year, number);

CREATE UNIQUE INDEX certificates_active_registry_uidx
    ON certificates (registry_id)
    WHERE deleted_at IS NULL;
