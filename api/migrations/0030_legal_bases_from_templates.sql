-- Przeniesienie podstaw prawnych z szablonów kursów do biblioteki (migracja 0029).
--
-- URUCHAMIAĆ DOPIERO PO WDROŻENIU KODU, KTÓRY ZNA ZNACZNIK {{ podstawa_prawna }}.
-- Starszy kod zostawiłby w miejscu znacznika pustkę - podstawa zniknęłaby z wydruków.
--
-- Próba na sucho (raport bez zapisu):
--   psql -v ON_ERROR_STOP=1 -c 'BEGIN' -f 0030_legal_bases_from_templates.sql -c 'ROLLBACK'
-- Wdrożenie:
--   psql --single-transaction -v ON_ERROR_STOP=1 -f 0030_legal_bases_from_templates.sql
--
-- Co robi:
--  1. Zakłada w bibliotece pięć podstaw używanych w szablonach, w poprawionym brzmieniu:
--     dwie literówki z szablonów ("Dz. U. z 20179", "6 października 2024 r." zamiast 2023)
--     i brak spacji w "poz.1632" / "poz.652". Poza tym słowo w słowo jak w szablonach.
--  2. W szablonach kursów i ich tłumaczeń zamienia pierwsze wystąpienie tekstu podstawy
--     (od "§ ..." do zamykającego nawiasu z Dz. U.) na znacznik {{ podstawa_prawna }}.
--     "na podstawie" / "Zaświadczenie wydano" zostaje w szablonie.
--  3. Kursowi, którego szablon cytuje podstawę, ustawia legal_basis_id - także wtedy,
--     gdy tekstu nie dało się bezpiecznie podmienić.
--  4. Każdą zmianę kursu zapisuje w historii zmian (audit_log).
--  5. Kończy się raportem dla każdego kursu.
--
-- Dopasowanie toleruje różne odstępy, &nbsp; i znaczniki HTML między słowami - autor
-- szablonu łamał zdanie na kilka akapitów o tym samym formatowaniu
-- (...Ministra</span></p><p ...><span ...>Gospodarki...). Wycięte razem z tekstem znaczniki
-- muszą się jednak bilansować: każdy zamknięty jest z powrotem otwarty. Tylko wtedy HTML
-- po podmianie pozostaje poprawny; zdanie scala się wtedy w jeden akapit (status
-- "scalono wiersze - sprawdź wygląd"). Gdy znaczniki się nie bilansują (np. pogrubienie
-- otwarte w środku zdania i zamknięte za nim), szablon zostaje nietknięty i trafia do
-- raportu jako "do ręcznej poprawki".
--
-- Skrypt jest idempotentny: drugie uruchomienie niczego nie zmienia (w szablonach nie ma
-- już tekstu do podmiany, a ustawionego legal_basis_id nie nadpisuje).

-- 1. Biblioteka -------------------------------------------------------------------------

INSERT INTO legal_bases (name, content) VALUES
    ('Szkolenia BHP — rozporządzenie MGiP z 27.07.2004',
     '§ 16 ust. 3 rozporządzenia Ministra Gospodarki i Pracy z dnia 27 lipca 2004 r. w sprawie szkolenia w dziedzinie bezpieczeństwa i higieny pracy (DzU nr 180, poz. 1860, z późn. zm.)'),
    ('Kształcenie ustawiczne — rozporządzenie MEN z 18.08.2017',
     '§ 18 ust. 2 rozporządzenia Ministra Edukacji Narodowej z dnia 18 sierpnia 2017 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2017 poz. 1632)'),
    ('Kształcenie ustawiczne — rozporządzenie MEN z 19.03.2019',
     '§ 22 ust. 3 rozporządzenia Ministra Edukacji Narodowej z dnia 19 marca 2019 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2019 poz. 652)'),
    ('Kształcenie ustawiczne — rozporządzenie MEiN z 06.10.2023',
     '§ 23 ust. 3 rozporządzenia Ministra Edukacji i Nauki z dnia 6 października 2023 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2023 poz. 2175)'),
    ('Diagności — rozporządzenie MIiR z 28.11.2014',
     'rozporządzenia Ministra Infrastruktury i Rozwoju z dnia 28 listopada 2014 r. w sprawie szkolenia i egzaminowania diagnostów oraz wzorów dokumentów z tym związanych (Dz. U. poz. 1836)')
ON CONFLICT (lower(btrim(name))) DO NOTHING;

-- 2. Warianty zapisu w szablonach -> podstawa ----------------------------------------------
-- Dosłowne brzmienia znalezione w szablonach, z pojedynczymi spacjami. Literówki to osobne
-- warianty wskazujące tę samą (poprawioną) podstawę. Kolejność = priorytet przy dopasowaniu.

-- Obiekty tymczasowe bez ON COMMIT DROP i z DROP IF EXISTS: skrypt ma działać tak samo
-- w transakcji (psql --single-transaction), poza nią i uruchomiony drugi raz w tej samej sesji.
DROP TABLE IF EXISTS pg_temp.legal_basis_variants, pg_temp.legal_basis_plan;

CREATE TEMP TABLE legal_basis_variants (
    priority int PRIMARY KEY,
    phrase text NOT NULL,
    basis_name text NOT NULL
);

INSERT INTO legal_basis_variants (priority, phrase, basis_name) VALUES
    (1, '§ 16 ust. 3 rozporządzenia Ministra Gospodarki i Pracy z dnia 27 lipca 2004 r. w sprawie szkolenia w dziedzinie bezpieczeństwa i higieny pracy (DzU nr 180, poz. 1860, z późn. zm.)',
        'Szkolenia BHP — rozporządzenie MGiP z 27.07.2004'),
    (2, '§ 18 ust. 2 rozporządzenia Ministra Edukacji Narodowej z dnia 18 sierpnia 2017 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2017 poz.1632)',
        'Kształcenie ustawiczne — rozporządzenie MEN z 18.08.2017'),
    (3, '§ 18 ust. 2 rozporządzenia Ministra Edukacji Narodowej z dnia 18 sierpnia 2017 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2017 poz. 1632)',
        'Kształcenie ustawiczne — rozporządzenie MEN z 18.08.2017'),
    (4, '§ 22 ust. 3 rozporządzenia Ministra Edukacji Narodowej z dnia 19 marca 2019 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 20179 poz. 652)',
        'Kształcenie ustawiczne — rozporządzenie MEN z 19.03.2019'),
    (5, '§ 22 ust. 3 rozporządzenia Ministra Edukacji Narodowej z dnia 19 marca 2019 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2019 poz.652)',
        'Kształcenie ustawiczne — rozporządzenie MEN z 19.03.2019'),
    (6, '§ 22 ust. 3 rozporządzenia Ministra Edukacji Narodowej z dnia 19 marca 2019 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2019 poz. 652)',
        'Kształcenie ustawiczne — rozporządzenie MEN z 19.03.2019'),
    (7, '§ 23 ust. 3 rozporządzenia Ministra Edukacji i Nauki z dnia 6 października 2024 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2023 poz. 2175)',
        'Kształcenie ustawiczne — rozporządzenie MEiN z 06.10.2023'),
    (8, '§ 23 ust. 3 rozporządzenia Ministra Edukacji i Nauki z dnia 6 października 2023 r. w sprawie kształcenia ustawicznego w formach pozaszkolnych (Dz. U. z 2023 poz. 2175)',
        'Kształcenie ustawiczne — rozporządzenie MEiN z 06.10.2023'),
    (9, 'rozporządzenia Ministra Infrastruktury i Rozwoju z dnia 28 listopada 2014 r. w sprawie szkolenia i egzaminowania diagnostów oraz wzorów dokumentów z tym związanych (Dz. U. poz. 1836)',
        'Diagności — rozporządzenie MIiR z 28.11.2014');

-- 3. Narzędzia dopasowania ---------------------------------------------------------------

-- Tekst szablonu bez znaczników, z &nbsp; jako spacją i pojedynczymi odstępami.
CREATE OR REPLACE FUNCTION pg_temp.legal_basis_plain(html text) RETURNS text
    LANGUAGE sql IMMUTABLE
AS $$
    SELECT btrim(regexp_replace(
        regexp_replace(replace(coalesce(html, ''), '&nbsp;', ' '), '<[^>]*>', ' ', 'g'),
        '\s+', ' ', 'g'))
$$;

-- Fraza -> wyrażenie regularne dopasowujące ją w HTML: znaki specjalne escapowane, a każda
-- spacja z frazy dopuszcza dowolny ciąg odstępów, &nbsp; i znaczników HTML.
CREATE OR REPLACE FUNCTION pg_temp.legal_basis_loose(phrase text) RETURNS text
    LANGUAGE sql IMMUTABLE
AS $$
    SELECT '(' || replace(
        regexp_replace(phrase, '([.^$*+?()\[\]{}|\\-])', '\\\1', 'g'),
        ' ', '(?:\s|&nbsp;|<[^>]*>)+') || ')'
$$;

-- Czy znaczniki wycinane razem z tekstem się bilansują: dla każdej nazwy tyle samo zamknięć
-- co otwarć. <br> i <wbr> są neutralne (to tylko łamanie wiersza).
CREATE OR REPLACE FUNCTION pg_temp.legal_basis_tags_balanced(fragment text) RETURNS boolean
    LANGUAGE plpgsql IMMUTABLE
AS $$
DECLARE
    tag text[];
    counts jsonb := '{}'::jsonb;
    name text;
BEGIN
    FOR tag IN SELECT regexp_matches(fragment, '<(/?)([A-Za-z][A-Za-z0-9]*)[^>]*>', 'g') LOOP
        name := lower(tag[2]);
        CONTINUE WHEN name IN ('br', 'wbr');
        counts := counts || jsonb_build_object(
            name,
            coalesce((counts ->> name)::int, 0) + CASE WHEN tag[1] = '/' THEN -1 ELSE 1 END);
    END LOOP;
    RETURN NOT EXISTS (SELECT 1 FROM jsonb_each_text(counts) WHERE value::int <> 0);
END;
$$;

-- 4. Analiza szablonów -------------------------------------------------------------------

CREATE TEMP TABLE legal_basis_plan AS
WITH templates AS (
    SELECT 'course'::text AS kind, c.id AS template_id, c.id AS course_id, c.certfrontpage AS html
    FROM courses c
    UNION ALL
    SELECT 'translation', t.id, t.course_id, t.cert_front_page
    FROM course_certificate_translations t
), first_variant AS (
    SELECT DISTINCT ON (tpl.kind, tpl.template_id)
        tpl.kind, tpl.template_id, tpl.course_id, tpl.html,
        v.phrase, lb.id AS basis_id, lb.content AS basis_content,
        substring(tpl.html from pg_temp.legal_basis_loose(v.phrase)) AS fragment
    FROM templates tpl
    JOIN legal_basis_variants v
      ON pg_temp.legal_basis_plain(tpl.html) LIKE '%' || v.phrase || '%'
    JOIN legal_bases lb ON lower(btrim(lb.name)) = lower(btrim(v.basis_name))
    ORDER BY tpl.kind, tpl.template_id, v.priority
)
SELECT
    kind, template_id, course_id, html, basis_id, basis_content, fragment,
    CASE
        WHEN fragment IS NULL OR NOT pg_temp.legal_basis_tags_balanced(fragment) THEN 'do ręcznej poprawki'
        WHEN fragment ~* '</(p|div|h[1-6]|li|tr|td)>|<br' THEN 'scalono wiersze - sprawdź wygląd'
        ELSE 'podmieniono'
    END AS outcome
FROM first_variant;

ALTER TABLE legal_basis_plan ADD COLUMN new_html text;
UPDATE legal_basis_plan
SET new_html = overlay(html placing '{{ podstawa_prawna }}' from strpos(html, fragment) for length(fragment))
WHERE outcome <> 'do ręcznej poprawki';

-- 5. Zapis --------------------------------------------------------------------------------

-- Historia zmian najpierw, póki "before" jest jeszcze w tabeli.
INSERT INTO audit_log (entity_type, entity_id, action, actor_user_name_snapshot, before_data, after_data, metadata)
SELECT
    'course', c.id, 'update', 'Migracja bazy (0030)',
    jsonb_build_object('certFrontPage', c.certfrontpage, 'legalBasisId', c.legal_basis_id),
    jsonb_build_object(
        'certFrontPage', coalesce(p.new_html, c.certfrontpage),
        'legalBasisId', coalesce(c.legal_basis_id, p.basis_id)),
    jsonb_build_object('operation', 'legal_basis_migration', 'outcome', p.outcome)
FROM legal_basis_plan p
JOIN courses c ON c.id = p.template_id
WHERE p.kind = 'course'
  AND (p.new_html IS NOT NULL OR c.legal_basis_id IS NULL);

UPDATE courses c
SET certfrontpage = coalesce(p.new_html, c.certfrontpage),
    legal_basis_id = coalesce(c.legal_basis_id, p.basis_id)
FROM legal_basis_plan p
WHERE p.kind = 'course' AND c.id = p.template_id
  AND (p.new_html IS NOT NULL OR c.legal_basis_id IS NULL);

UPDATE course_certificate_translations t
SET cert_front_page = p.new_html
FROM legal_basis_plan p
WHERE p.kind = 'translation' AND t.id = p.template_id AND p.new_html IS NOT NULL;

-- Kurs, w którego szablonie podstawa była tylko w tłumaczeniu, też dostaje przypisanie.
UPDATE courses c
SET legal_basis_id = p.basis_id
FROM legal_basis_plan p
WHERE p.kind = 'translation' AND c.id = p.course_id AND c.legal_basis_id IS NULL;

-- 6. Raport -------------------------------------------------------------------------------
-- "tekst bez zmian" porównuje tekst szablonu przed i po migracji (po podstawieniu treści
-- podstawy w miejsce znacznika), z pominięciem odstępów. "nie" oznacza poprawioną literówkę.

SELECT
    c.id AS kurs,
    c.symbol,
    CASE p.kind WHEN 'translation' THEN 'tłumaczenie' ELSE 'szablon' END AS gdzie,
    lb.name AS podstawa,
    coalesce(p.outcome, CASE WHEN c.legal_basis_id IS NULL THEN 'bez podstawy' ELSE 'już przypisana' END) AS wynik,
    CASE
        WHEN p.new_html IS NULL THEN '-'
        WHEN regexp_replace(pg_temp.legal_basis_plain(p.html), '\s', '', 'g')
           = regexp_replace(pg_temp.legal_basis_plain(replace(p.new_html, '{{ podstawa_prawna }}', p.basis_content)), '\s', '', 'g')
        THEN 'tak'
        ELSE 'nie'
    END AS "tekst bez zmian"
FROM courses c
LEFT JOIN legal_basis_plan p ON p.course_id = c.id
LEFT JOIN legal_bases lb ON lb.id = coalesce(p.basis_id, c.legal_basis_id)
ORDER BY
    CASE coalesce(p.outcome, 'bez podstawy')
        WHEN 'do ręcznej poprawki' THEN 0
        WHEN 'scalono wiersze - sprawdź wygląd' THEN 1
        WHEN 'bez podstawy' THEN 2
        ELSE 3
    END,
    c.id;

DROP TABLE pg_temp.legal_basis_variants, pg_temp.legal_basis_plan;
