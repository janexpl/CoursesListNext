-- Identyfikator platformy e-learningowej (externalId) dla kursantów i firm.
--
-- Platforma zakłada kursanta i firmę przez PUT /students/by-external-id/{externalId}
-- i PUT /companies/by-external-id/{externalId} i może to wywołanie powtarzać bez
-- tworzenia duplikatów. external_id to nieprzezroczysty napis platformy (w praktyce
-- UUID) - API go nie interpretuje.
--
--   - kolumna jest nullowalna: rekordy zakładane w aplikacji webowej go nie mają,
--   - unikalność częściowa: wartość niepusta jest unikalna, NULL-i może być dowolnie wiele,
--   - CHECK: 1-64 znaki, żeby pusty string nie udawał "braku" obok NULL.
--
-- Wpływ na istniejące dane: brak. Wszystkie istniejące kursanci i firmy dostają
-- external_id = NULL; ADD COLUMN bez wartości domyślnej nie przepisuje tabeli.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

ALTER TABLE students
    ADD COLUMN external_id text,
    ADD CONSTRAINT students_external_id_length CHECK (char_length(external_id) BETWEEN 1 AND 64);

CREATE UNIQUE INDEX students_external_id_uidx
    ON students (external_id)
    WHERE external_id IS NOT NULL;

ALTER TABLE companies
    ADD COLUMN external_id text,
    ADD CONSTRAINT companies_external_id_length CHECK (char_length(external_id) BETWEEN 1 AND 64);

CREATE UNIQUE INDEX companies_external_id_uidx
    ON companies (external_id)
    WHERE external_id IS NOT NULL;
