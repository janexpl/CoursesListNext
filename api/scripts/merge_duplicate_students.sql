-- Scalanie zduplikowanych kursantów.
--
-- Duplikat to kursanci o tym samym imieniu, nazwisku i dacie urodzenia, różniący się
-- tylko wielkością liter lub spacjami na brzegach - ta sama reguła, którą wymusza
-- migracja 0018. Po scaleniu 0018 da się uruchomić.
--
-- UŻYCIE
--   Podgląd (domyślnie): wykonuje całe scalanie razem z kontrolami poprawności, wypisuje
--   raport i wycofuje transakcję - nic nie zostaje zapisane.
--     psql -v ON_ERROR_STOP=1 -f api/scripts/merge_duplicate_students.sql
--
--   Zapis:
--     psql -v ON_ERROR_STOP=1 -v apply=true -f api/scripts/merge_duplicate_students.sql
--
--   Na czas działania tabele students, certificates i training_journal_attendees są
--   blokowane dla zapisu (także w podglądzie) - trwa to kilka sekund. Skrypt można
--   uruchomić ponownie: gdy duplikatów nie ma, nic nie robi.
--
-- REGUŁY SCALANIA (w każdej grupie)
--   - Zostaje rekord najstarszy (najniższe id): zachowuje ciągłość historii zmian.
--   - Imię i nazwisko: bez spacji na brzegach; zapis z mieszaną wielkością liter ("Michał")
--     ma pierwszeństwo przed pisanym samymi wielkimi lub małymi literami, a przy remisie
--     wygrywa najnowszy rekord.
--   - Pozostałe pola (PESEL, firma, miejsce urodzenia, drugie imię, adres, telefon):
--     wartość z najnowszego rekordu, który ją ma. Nowszy wpis częściej odzwierciedla
--     aktualne dane. PESEL nie jest oceniany pod kątem poprawności - u obcokrajowców pole
--     zawiera numer paszportu lub innego dokumentu.
--   - Zaświadczenia i udziały w dziennikach są przepinane na pozostający rekord. Kopie
--     danych w zaświadczeniach (student_*_snapshot) NIE są zmieniane - to treść wydanych
--     dokumentów.
--   Raport wypisuje każdą grupę, w której wartości pól się różniły, i wybraną wartość -
--   przejrzyj go przed uruchomieniem z apply=true.
--
-- ŚLAD I COFANIE
--   - Tabela students_merge_backup przechowuje pełne wiersze sprzed scalenia (pozostające
--     i usunięte) oraz identyfikatory przepiętych zaświadczeń i uczestników dzienników.
--     Pozwala to odtworzyć stan ręcznie. Usuń ją, gdy wynik zostanie zweryfikowany.
--   - Historia zmian (audit_log) dostaje wpis "update" dla pozostającego kursanta
--     i "delete" dla każdego usuniętego, widoczne w aplikacji.

\set ON_ERROR_STOP on
\if :{?apply}
\else
    \set apply false
\endif

BEGIN;

LOCK TABLE students, certificates, training_journal_attendees IN SHARE ROW EXCLUSIVE MODE;

-- Członkowie grup duplikatów.
CREATE TEMP TABLE merge_members ON COMMIT DROP AS
SELECT
    s.*,
    min(s.id) OVER grp AS survivor_id,
    dense_rank() OVER (ORDER BY lower(btrim(s.lastname)), lower(btrim(s.firstname)), s.birthdate) AS group_no
FROM students s
WHERE (lower(btrim(s.firstname)), lower(btrim(s.lastname)), s.birthdate) IN (
    SELECT lower(btrim(firstname)), lower(btrim(lastname)), birthdate
    FROM students
    GROUP BY 1, 2, 3
    HAVING count(*) > 1
)
WINDOW grp AS (PARTITION BY lower(btrim(s.firstname)), lower(btrim(s.lastname)), s.birthdate);

-- Docelowe wartości pozostającego rekordu. "Najnowszy" = najwyższe id.
CREATE TEMP TABLE merge_plan ON COMMIT DROP AS
SELECT
    group_no,
    survivor_id,
    array_agg(id ORDER BY id) FILTER (WHERE id <> survivor_id) AS removed_ids,
    -- Zapis pisany samymi wielkimi lub małymi literami ("MICHAŁ", "michał") przegrywa
    -- z zapisem mieszanym ("Michał"); przy remisie wygrywa najnowszy rekord.
    (array_agg(btrim(firstname) ORDER BY btrim(firstname) IN (upper(btrim(firstname)), lower(btrim(firstname))), id DESC))[1] AS firstname,
    (array_agg(btrim(lastname) ORDER BY btrim(lastname) IN (upper(btrim(lastname)), lower(btrim(lastname))), id DESC))[1] AS lastname,
    min(birthdate) AS birthdate,
    (array_agg(btrim(pesel) ORDER BY id DESC) FILTER (WHERE nullif(btrim(pesel), '') IS NOT NULL))[1] AS pesel,
    (array_agg(company_id ORDER BY id DESC) FILTER (WHERE company_id IS NOT NULL))[1] AS company_id,
    (array_agg(btrim(birthplace) ORDER BY id DESC) FILTER (WHERE nullif(btrim(birthplace), '') IS NOT NULL))[1] AS birthplace,
    (array_agg(btrim(secondname) ORDER BY id DESC) FILTER (WHERE nullif(btrim(secondname), '') IS NOT NULL))[1] AS secondname,
    (array_agg(btrim(addressstreet) ORDER BY id DESC) FILTER (WHERE nullif(btrim(addressstreet), '') IS NOT NULL))[1] AS addressstreet,
    (array_agg(btrim(addresscity) ORDER BY id DESC) FILTER (WHERE nullif(btrim(addresscity), '') IS NOT NULL))[1] AS addresscity,
    (array_agg(btrim(addresszip) ORDER BY id DESC) FILTER (WHERE nullif(btrim(addresszip), '') IS NOT NULL))[1] AS addresszip,
    (array_agg(btrim(telephoneno) ORDER BY id DESC) FILTER (WHERE nullif(btrim(telephoneno), '') IS NOT NULL))[1] AS telephoneno
FROM merge_members
GROUP BY group_no, survivor_id;

-- ---------------------------------------------------------------- raport

\echo
\echo '=== Podsumowanie ==='
SELECT
    (SELECT count(*) FROM merge_plan) AS grup_duplikatow,
    (SELECT count(*) FROM merge_members) AS kursantow_w_grupach,
    (SELECT coalesce(sum(cardinality(removed_ids)), 0) FROM merge_plan) AS rekordow_do_usuniecia,
    (SELECT count(*) FROM certificates WHERE student_id IN (SELECT unnest(removed_ids) FROM merge_plan)) AS zaswiadczen_do_przepiecia,
    (SELECT count(*) FROM training_journal_attendees WHERE student_id IN (SELECT unnest(removed_ids) FROM merge_plan)) AS uczestnikow_dziennikow_do_przepiecia;

\echo
\echo '=== Grupy, w których wartości się różniły (wybrana wartość po strzałce) ==='
SELECT
    p.group_no AS grupa,
    p.survivor_id AS zostaje,
    array_to_string(p.removed_ids, ',') AS usuwane,
    p.firstname || ' ' || p.lastname AS osoba,
    CASE WHEN count(DISTINCT m.firstname || '|' || m.lastname) > 1
         THEN string_agg(DISTINCT '"' || m.firstname || ' ' || m.lastname || '"', ' / ') || ' -> "' || p.firstname || ' ' || p.lastname || '"' END AS zapis,
    CASE WHEN count(DISTINCT nullif(btrim(m.pesel), '')) > 1
         THEN string_agg(DISTINCT coalesce(btrim(m.pesel), '∅'), ' / ') || ' -> ' || coalesce(p.pesel, '∅') END AS pesel,
    CASE WHEN count(DISTINCT m.company_id) > 1
         THEN string_agg(DISTINCT coalesce(c.name, '∅'), ' / ') || ' -> ' || coalesce((SELECT name FROM companies WHERE id = p.company_id), '∅') END AS firma,
    CASE WHEN count(DISTINCT lower(btrim(m.birthplace))) > 1
         THEN string_agg(DISTINCT coalesce(btrim(m.birthplace), '∅'), ' / ') || ' -> ' || coalesce(p.birthplace, '∅') END AS miejsce_urodzenia
FROM merge_plan p
JOIN merge_members m ON m.survivor_id = p.survivor_id
LEFT JOIN companies c ON c.id = m.company_id
GROUP BY p.group_no, p.survivor_id, p.removed_ids, p.firstname, p.lastname, p.pesel, p.company_id, p.birthplace
HAVING count(DISTINCT m.firstname || '|' || m.lastname) > 1
    OR count(DISTINCT nullif(btrim(m.pesel), '')) > 1
    OR count(DISTINCT m.company_id) > 1
    OR count(DISTINCT lower(btrim(m.birthplace))) > 1
ORDER BY p.group_no;

-- ---------------------------------------------------------------- kontrole przed scaleniem

DO $$
DECLARE
    conflicts integer;
BEGIN
    -- Dwa rekordy tej samej osoby w jednym dzienniku: przepięcie naruszyłoby unikalność
    -- (journal_id, student_id), a scalenie obecności wymaga ręcznej decyzji.
    SELECT count(*) INTO conflicts
    FROM (
        SELECT a.journal_id, m.survivor_id
        FROM training_journal_attendees a
        JOIN merge_members m ON m.id = a.student_id
        GROUP BY a.journal_id, m.survivor_id
        HAVING count(*) > 1
    ) x;

    IF conflicts > 0 THEN
        RAISE EXCEPTION
            'kursanci z % grup są uczestnikami tego samego dziennika pod różnymi rekordami - usuń zbędnego uczestnika ręcznie i uruchom skrypt ponownie',
            conflicts;
    END IF;
END $$;

-- ---------------------------------------------------------------- scalanie

CREATE TABLE IF NOT EXISTS students_merge_backup (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    merged_at timestamptz NOT NULL,
    kind text NOT NULL CHECK (kind IN ('survivor', 'removed')),
    student_id bigint NOT NULL,
    survivor_id bigint NOT NULL,
    row_before jsonb NOT NULL,
    certificate_ids bigint[] NOT NULL DEFAULT '{}',
    attendee_ids bigint[] NOT NULL DEFAULT '{}'
);

-- Migawka w formacie, którego używa API w historii zmian (StudentDetailsDTO).
CREATE TEMP TABLE merge_snapshots ON COMMIT DROP AS
SELECT
    m.id,
    m.survivor_id,
    jsonb_build_object(
        'id', m.id,
        'firstName', m.firstname,
        'lastName', m.lastname,
        'secondName', m.secondname,
        'birthDate', to_char(m.birthdate, 'YYYY-MM-DD'),
        'birthPlace', m.birthplace,
        'pesel', m.pesel,
        'addressStreet', m.addressstreet,
        'addressCity', m.addresscity,
        'addressZip', m.addresszip,
        'telephone', m.telephoneno,
        'company', CASE WHEN c.id IS NULL THEN NULL ELSE jsonb_build_object('id', c.id, 'name', c.name) END
    ) AS snapshot
FROM merge_members m
LEFT JOIN companies c ON c.id = m.company_id;

INSERT INTO students_merge_backup (merged_at, kind, student_id, survivor_id, row_before, certificate_ids, attendee_ids)
SELECT
    now(),
    CASE WHEN m.id = m.survivor_id THEN 'survivor' ELSE 'removed' END,
    m.id,
    m.survivor_id,
    to_jsonb(s),
    coalesce((SELECT array_agg(c.id ORDER BY c.id) FROM certificates c WHERE c.student_id = m.id), '{}'),
    coalesce((SELECT array_agg(a.id ORDER BY a.id) FROM training_journal_attendees a WHERE a.student_id = m.id), '{}')
FROM merge_members m
JOIN students s ON s.id = m.id;

UPDATE certificates c
SET student_id = m.survivor_id
FROM merge_members m
WHERE c.student_id = m.id
  AND m.id <> m.survivor_id;

UPDATE training_journal_attendees a
SET student_id = m.survivor_id
FROM merge_members m
WHERE a.student_id = m.id
  AND m.id <> m.survivor_id;

INSERT INTO audit_log (entity_type, entity_id, action, actor_user_id, actor_user_email_snapshot, actor_user_name_snapshot, request_id, before_data, after_data, metadata)
SELECT
    'student', m.id, 'delete', NULL, NULL, 'Skrypt: scalanie duplikatów kursantów', NULL,
    sn.snapshot, NULL,
    jsonb_build_object('source', 'merge_duplicate_students', 'mergedInto', m.survivor_id)
FROM merge_members m
JOIN merge_snapshots sn ON sn.id = m.id
WHERE m.id <> m.survivor_id;

-- Usuwamy przed aktualizacją pozostającego rekordu: po przycięciu spacji jego imię
-- i nazwisko mogłyby być identyczne z usuwanym i naruszyć students_person_unique.
DELETE FROM students s
USING merge_members m
WHERE s.id = m.id
  AND m.id <> m.survivor_id;

UPDATE students s
SET firstname     = p.firstname,
    lastname      = p.lastname,
    pesel         = p.pesel,
    company_id    = p.company_id,
    birthplace    = p.birthplace,
    secondname    = p.secondname,
    addressstreet = p.addressstreet,
    addresscity   = p.addresscity,
    addresszip    = p.addresszip,
    telephoneno   = p.telephoneno
FROM merge_plan p
WHERE s.id = p.survivor_id;

INSERT INTO audit_log (entity_type, entity_id, action, actor_user_id, actor_user_email_snapshot, actor_user_name_snapshot, request_id, before_data, after_data, metadata)
SELECT
    'student', p.survivor_id, 'update', NULL, NULL, 'Skrypt: scalanie duplikatów kursantów', NULL,
    before.snapshot,
    jsonb_build_object(
        'id', s.id,
        'firstName', s.firstname,
        'lastName', s.lastname,
        'secondName', s.secondname,
        'birthDate', to_char(s.birthdate, 'YYYY-MM-DD'),
        'birthPlace', s.birthplace,
        'pesel', s.pesel,
        'addressStreet', s.addressstreet,
        'addressCity', s.addresscity,
        'addressZip', s.addresszip,
        'telephone', s.telephoneno,
        'company', CASE WHEN c.id IS NULL THEN NULL ELSE jsonb_build_object('id', c.id, 'name', c.name) END
    ),
    jsonb_build_object('source', 'merge_duplicate_students', 'mergedStudentIds', to_jsonb(p.removed_ids))
FROM merge_plan p
JOIN merge_snapshots before ON before.id = p.survivor_id
JOIN students s ON s.id = p.survivor_id
LEFT JOIN companies c ON c.id = s.company_id;

-- ---------------------------------------------------------------- kontrole po scaleniu

CREATE TEMP TABLE merge_counts_before ON COMMIT DROP AS
SELECT
    (SELECT count(*) FROM students_merge_backup b WHERE b.merged_at = now()) AS backup_rows,
    (SELECT coalesce(sum(cardinality(certificate_ids)), 0) FROM students_merge_backup b WHERE b.merged_at = now()) AS certificates_in_groups,
    (SELECT coalesce(sum(cardinality(attendee_ids)), 0) FROM students_merge_backup b WHERE b.merged_at = now()) AS attendees_in_groups;

DO $$
DECLARE
    remaining_groups integer;
    orphaned integer;
    certificates_now integer;
    attendees_now integer;
    expected record;
BEGIN
    SELECT count(*) INTO remaining_groups
    FROM (
        SELECT 1 FROM students
        GROUP BY lower(btrim(firstname)), lower(btrim(lastname)), birthdate
        HAVING count(*) > 1
    ) x;
    IF remaining_groups > 0 THEN
        RAISE EXCEPTION 'po scaleniu wciąż istnieje % grup duplikatów', remaining_groups;
    END IF;

    SELECT count(*) INTO orphaned
    FROM students_merge_backup b
    WHERE b.merged_at = now() AND b.kind = 'removed'
      AND EXISTS (SELECT 1 FROM students s WHERE s.id = b.student_id);
    IF orphaned > 0 THEN
        RAISE EXCEPTION 'nie usunięto % scalonych rekordów', orphaned;
    END IF;

    SELECT * INTO expected FROM merge_counts_before;

    -- Wszystkie zaświadczenia i udziały członków grup muszą należeć teraz do pozostających rekordów.
    SELECT count(*) INTO certificates_now
    FROM certificates
    WHERE student_id IN (SELECT survivor_id FROM students_merge_backup WHERE merged_at = now() AND kind = 'survivor');
    IF certificates_now <> expected.certificates_in_groups THEN
        RAISE EXCEPTION 'liczba zaświadczeń scalonych kursantów się nie zgadza: jest %, oczekiwano %',
            certificates_now, expected.certificates_in_groups;
    END IF;

    SELECT count(*) INTO attendees_now
    FROM training_journal_attendees
    WHERE student_id IN (SELECT survivor_id FROM students_merge_backup WHERE merged_at = now() AND kind = 'survivor');
    IF attendees_now <> expected.attendees_in_groups THEN
        RAISE EXCEPTION 'liczba uczestników dzienników scalonych kursantów się nie zgadza: jest %, oczekiwano %',
            attendees_now, expected.attendees_in_groups;
    END IF;
END $$;

\echo
\echo '=== Po scaleniu ==='
SELECT
    (SELECT count(*) FROM students) AS kursantow,
    (SELECT count(*) FROM (SELECT 1 FROM students GROUP BY lower(btrim(firstname)), lower(btrim(lastname)), birthdate HAVING count(*) > 1) x) AS pozostalych_grup_duplikatow,
    (SELECT count(*) FROM students_merge_backup WHERE merged_at = now()) AS wierszy_kopii_zapasowej;

\if :apply
    COMMIT;
    \echo
    \echo 'ZAPISANO. Kopia wierszy sprzed scalenia: tabela students_merge_backup.'
    \echo 'Teraz można uruchomić migrację 0018_students_person_normalized_unique.sql.'
\else
    ROLLBACK;
    \echo
    \echo 'PODGLĄD - wszystkie kontrole przeszły, ale nic nie zapisano.'
    \echo 'Aby zapisać: psql -v ON_ERROR_STOP=1 -v apply=true -f api/scripts/merge_duplicate_students.sql'
\endif
