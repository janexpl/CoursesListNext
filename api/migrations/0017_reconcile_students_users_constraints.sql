-- Uzgadnia ograniczenia tabel students i users z modelem aplikacji.
--
-- Bazy założone ze starego schematu zachowały ograniczenia, których żadna migracja
-- nie usunęła, obok tych dodanych później:
--   - students.company_id NOT NULL, choć API i frontend traktują firmę jako opcjonalną
--     (utworzenie kursanta bez firmy kończyło się błędem 500),
--   - dwa klucze obce na students.company_id o sprzecznym zachowaniu przy usuwaniu
--     firmy: fk_company (ON DELETE RESTRICT) i students_company_id_fkey z migracji 0013
--     (ON DELETE SET NULL),
--   - dwa identyczne ograniczenia unikalności users.email: unique_email i
--     users_email_unique z migracji 0004.
--
-- Decyzje:
--   - kursant może nie mieć firmy; usunięcie firmy zostawia kursantów bez firmy
--     (zachowanie z migracji 0013),
--   - duplikaty osób są blokowane. Ta migracja utrwala w historii migracji istniejącą
--     regułę UNIQUE (firstname, lastname, birthdate) pod jednoznaczną nazwą. Reguła
--     niezależna od wielkości liter i spacji na brzegach wymaga wcześniejszego
--     uporządkowania istniejących duplikatów - patrz 0018.
--
-- Migracja nie zmienia danych. Każdy krok jest warunkowy, więc działa zarówno na bazie
-- ze starym schematem, jak i na bazie zbudowanej z internal/db/schema.sql.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

-- 1. Firma kursanta jest opcjonalna.
ALTER TABLE students ALTER COLUMN company_id DROP NOT NULL;

-- 2. Jeden klucz obcy students.company_id - ten z migracji 0013.
ALTER TABLE students DROP CONSTRAINT IF EXISTS fk_company;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'students'::regclass AND conname = 'students_company_id_fkey'
    ) THEN
        ALTER TABLE students
            ADD CONSTRAINT students_company_id_fkey
            FOREIGN KEY (company_id) REFERENCES companies(id) ON DELETE SET NULL;
    END IF;
END $$;

-- 3. Unikalność osoby pod jednoznaczną nazwą. Stara nazwa sugeruje (lastname, birthdate),
--    ale ograniczenie obejmuje też imię - przemianowujemy je tylko po sprawdzeniu definicji,
--    żeby nie nadać mylącej nazwy innej regule.
DO $$
DECLARE
    legacy_definition text;
BEGIN
    SELECT pg_get_constraintdef(oid) INTO legacy_definition
    FROM pg_constraint
    WHERE conrelid = 'students'::regclass AND conname = 'unique_user_lastname_birthdate';

    IF legacy_definition IS NOT NULL THEN
        IF legacy_definition <> 'UNIQUE (firstname, lastname, birthdate)' THEN
            RAISE EXCEPTION
                'unique_user_lastname_birthdate ma nieoczekiwaną definicję "%" - wymagana ręczna weryfikacja',
                legacy_definition;
        END IF;
        ALTER TABLE students
            RENAME CONSTRAINT unique_user_lastname_birthdate TO students_person_unique;
    ELSIF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'students'::regclass AND conname = 'students_person_unique'
    ) THEN
        ALTER TABLE students
            ADD CONSTRAINT students_person_unique UNIQUE (firstname, lastname, birthdate);
    END IF;
END $$;

-- 4. Indeks pod sprawdzanie duplikatów bez rozróżniania wielkości liter i spacji na brzegach.
--    API wykonuje to sprawdzenie przy każdym zapisie kursanta; bez indeksu każde byłoby
--    pełnym przeglądem tabeli. Migracja 0018 zastępuje go indeksem unikalnym.
CREATE INDEX IF NOT EXISTS students_person_lookup_idx
    ON students (lower(btrim(lastname)), birthdate);

-- 5. Jedno ograniczenie unikalności users.email - to z migracji 0004.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'users'::regclass AND conname = 'users_email_unique'
    ) THEN
        ALTER TABLE users DROP CONSTRAINT IF EXISTS unique_email;
    ELSIF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'users'::regclass AND conname = 'unique_email'
    ) THEN
        ALTER TABLE users RENAME CONSTRAINT unique_email TO users_email_unique;
    ELSE
        ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);
    END IF;
END $$;
