-- Katalog kursów dla platformy e-learningowej: znacznik zmian i flaga dostarczania.
--
-- 1. courses.updated_at - moment ostatniej zmiany treści kursu, na którym opiera się
--    filtr ?updatedSince w GET /courses i GET /courses/details. Podnoszą go wyzwalacze:
--      - zmiana dowolnej kolumny kursu (w tym delivered_by_platform),
--      - dodanie, zmiana albo usunięcie tłumaczenia zaświadczenia.
--    Zapis bez faktycznej zmiany treści (np. PATCH z tymi samymi danymi, upsert
--    identycznego tłumaczenia) znacznika nie podnosi. Wartość to clock_timestamp()
--    z chwili zapisu.
-- 2. courses.delivered_by_platform - czy kurs jest oferowany przez platformę
--    (domyślnie false). Decyzję podejmuje CoursesList.
--
-- Wpływ na istniejące dane:
--   - wszystkie istniejące kursy dostają updated_at = czas uruchomienia migracji (DEFAULT
--     now() jest stały w obrębie transakcji, więc ADD COLUMN nie przepisuje tabeli).
--     Pierwsza synchronizacja z updatedSince sprzed migracji zwróci więc wszystkie kursy,
--   - wszystkie istniejące kursy dostają delivered_by_platform = false; kursy dostarczane
--     przez platformę trzeba oznaczyć w aplikacji webowej,
--   - pozostałe kolumny i tłumaczenia nie są zmieniane.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

ALTER TABLE courses
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN delivered_by_platform boolean NOT NULL DEFAULT false;

CREATE INDEX courses_updated_at_idx ON courses (updated_at);

-- Zmiana treści kursu. json nie ma operatora równości, więc wiersze porównujemy jako jsonb.
CREATE FUNCTION courses_touch_updated_at() RETURNS trigger
    LANGUAGE plpgsql
AS $$
BEGIN
    IF (to_jsonb(NEW) - 'updated_at') IS DISTINCT FROM (to_jsonb(OLD) - 'updated_at') THEN
        NEW.updated_at := clock_timestamp();
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER courses_touch_updated_at
    BEFORE UPDATE ON courses
    FOR EACH ROW
    EXECUTE FUNCTION courses_touch_updated_at();

-- Zmiana tłumaczenia podnosi znacznik kursu. UPDATE ustawia updated_at jawnie - wyzwalacz
-- na courses go nie nadpisuje, bo poza updated_at wiersz kursu się nie zmienia.
CREATE FUNCTION course_translations_touch_course() RETURNS trigger
    LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE'
        AND (to_jsonb(NEW) - 'updated_at' - 'created_at') IS NOT DISTINCT FROM (to_jsonb(OLD) - 'updated_at' - 'created_at') THEN
        RETURN NULL;
    END IF;

    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        UPDATE courses SET updated_at = clock_timestamp() WHERE id = OLD.course_id;
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') AND (TG_OP = 'INSERT' OR NEW.course_id <> OLD.course_id) THEN
        UPDATE courses SET updated_at = clock_timestamp() WHERE id = NEW.course_id;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER course_translations_touch_course
    AFTER INSERT OR UPDATE OR DELETE ON course_certificate_translations
    FOR EACH ROW
    EXECUTE FUNCTION course_translations_touch_course();
