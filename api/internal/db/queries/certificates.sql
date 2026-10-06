-- name: ListCertificates :many
SELECT
    c.id,
    c.date,
    c.student_firstname_snapshot AS student_firstname,
    c.student_lastname_snapshot AS student_lastname,
    c.company_name_snapshot AS company_name,
    c.course_name_snapshot AS course_name,
    c.course_symbol_snapshot AS course_symbol,
    r.year AS registry_year,
    r.number::bigint AS registry_number,
    c.coursedatestart AS course_date_start,
    c.coursedateend AS course_date_end,
    c.language_code,
    COALESCE(
        CASE
            WHEN c.coursedateend IS NOT NULL
                AND c.course_expiry_time_snapshot IS NOT NULL
                AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
            THEN TO_CHAR(c.coursedateend + c.course_expiry_time_snapshot::int * 365, 'YYYY-MM-DD')
            ELSE NULL::text
        END,
        ''
    ) AS expiry_date,
    c.revoked_at,
    c.duplicate_issued_at,
    c.renewed_at
FROM certificates c
JOIN registries r ON r.id = c.registry_id
WHERE
    (
        sqlc.narg(search)::text IS NULL
        OR COALESCE(c.student_firstname_snapshot, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(c.student_lastname_snapshot, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(c.company_name_snapshot, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(c.course_name_snapshot, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(c.course_symbol_snapshot, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR (
            r.number::bigint::text || '/' || c.course_symbol_snapshot || '/' || r.year::text
        ) ILIKE '%' || sqlc.narg(search)::text || '%'
    )
    AND (sqlc.narg(date_from)::date IS NULL OR c.date >= sqlc.narg(date_from)::date)
    AND (sqlc.narg(date_to)::date IS NULL OR c.date <= sqlc.narg(date_to)::date)
    AND c.deleted_at IS NULL
ORDER BY c.date DESC, c.id DESC
LIMIT sqlc.arg(limit_count);

-- UWAGA: service.go robi konwersję strukturalną GetCertificateByIDRow(UpdateCertificateRow),
-- więc oba zapytania muszą zwracać identyczną listę kolumn - te same nazwy, typy
-- i KOLEJNOŚĆ. Nowe kolumny dopisuj na końcu obu list naraz; rozjechana kolejność przy
-- zgodnych typach (renewed_at i duplicate_issued_at są oba timestamptz) byłaby cichą
-- podmianą pól, której kompilator nie wyłapie.
-- name: GetCertificateByID :one
SELECT
    c.id,
    c.date,
    c.student_id,
    c.student_firstname_snapshot AS student_firstname,
    c.student_secondname_snapshot AS student_secondname,
    c.student_lastname_snapshot AS student_lastname,
    c.student_birthdate_snapshot AS student_birthdate,
    c.student_birthplace_snapshot AS student_birthplace,
    c.student_pesel_snapshot AS student_pesel,
    c.company_name_snapshot AS company_name,
    c.coursedatestart AS course_date_start,
    c.coursedateend AS course_date_end,
    r.id AS registry_id,
    r.year AS registry_year,
    r.number::bigint AS registry_number,
    r.course_id AS course_id,
    c.course_name_snapshot AS course_name,
    c.course_symbol_snapshot AS course_symbol,
    c.course_expiry_time_snapshot AS course_expiry_time,
    c.course_program_snapshot::text AS course_program,
    c.cert_front_page_snapshot AS cert_front_page,
    c.language_code,
    tja.id AS journal_attendee_id,
    tj.id AS journal_id,
    tj.title AS journal_title,
    tj.status AS journal_status,
    COALESCE(
        CASE
            WHEN c.coursedateend IS NOT NULL
                AND c.course_expiry_time_snapshot IS NOT NULL
                AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
            THEN TO_CHAR(c.coursedateend + c.course_expiry_time_snapshot::int * 365, 'YYYY-MM-DD')
            ELSE NULL::text
        END,
        ''
    ) AS expiry_date,
    c.verification_code,
    c.revoked_at,
    c.revoke_reason,
    c.duplicate_reason,
    c.duplicate_issued_at,
    c.idempotency_key,
    c.renewed_at,
    c.renewed_by_certificate_id,
    -- Wskazanie odwrotne: czego przedłużeniem jest ten dokument. Dołączone przez LEFT JOIN,
    -- a nie podzapytaniem skalarnym, bo sqlc otypowałby podzapytanie jako wartość
    -- nienullowalną i skanowanie NULL-a kończyłoby się błędem. Indeks
    -- certificates_renewed_by_certificate_id_uidx gwarantuje najwyżej jeden taki wiersz,
    -- więc złączenie nie powiela wyniku.
    prev.id AS renewal_of_certificate_id
FROM certificates c
LEFT JOIN certificates prev ON prev.renewed_by_certificate_id = c.id AND prev.deleted_at IS NULL
LEFT JOIN training_journal_attendees tja ON tja.certificate_id = c.id
LEFT JOIN training_journals tj ON tj.id = tja.journal_id
JOIN registries r ON r.id = c.registry_id
WHERE c.id = $1
  AND c.deleted_at IS NULL;

-- name: CreateCertificate :one
INSERT INTO certificates (
    date,
    student_id,
    coursedatestart,
    coursedateend,
    registry_id,
    language_code,
    student_firstname_snapshot,
    student_secondname_snapshot,
    student_lastname_snapshot,
    student_birthdate_snapshot,
    student_birthplace_snapshot,
    student_pesel_snapshot,
    company_name_snapshot,
    company_id_snapshot,
    course_name_snapshot,
    course_symbol_snapshot,
    course_expiry_time_snapshot,
    course_program_snapshot,
    cert_front_page_snapshot
) VALUES (
    sqlc.arg(date),
    sqlc.arg(student_id),
    sqlc.arg(course_date_start),
    sqlc.arg(course_date_end),
    sqlc.arg(registry_id),
    sqlc.arg(language_code),
    sqlc.arg(student_firstname_snapshot),
    sqlc.arg(student_secondname_snapshot),
    sqlc.arg(student_lastname_snapshot),
    sqlc.arg(student_birthdate_snapshot),
    sqlc.arg(student_birthplace_snapshot),
    sqlc.arg(student_pesel_snapshot),
    sqlc.arg(company_name_snapshot),
    sqlc.arg(company_id_snapshot),
    sqlc.arg(course_name_snapshot),
    sqlc.arg(course_symbol_snapshot),
    sqlc.arg(course_expiry_time_snapshot),
    sqlc.arg(course_program_snapshot),
    sqlc.arg(cert_front_page_snapshot)
)
RETURNING id, verification_code;

-- name: ListCertificatesByStudentID :many
  SELECT
      c.id,
      c.date,
	      c.course_name_snapshot AS course_name,
	      c.course_symbol_snapshot AS course_symbol,
      r.year AS registry_year,
      r.number::bigint AS registry_number,
      c.coursedatestart AS course_date_start,
      c.coursedateend AS course_date_end,
      COALESCE(CASE
          WHEN c.coursedateend IS NOT NULL
	              AND c.course_expiry_time_snapshot IS NOT NULL
	              AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
	          THEN TO_CHAR(c.coursedateend + c.course_expiry_time_snapshot::int * 365, 'YYYY-MM-DD')
          ELSE NULL::text
      END, '') AS expiry_date,
      c.revoked_at,
      c.duplicate_issued_at,
      c.renewed_at
  FROM certificates c
  JOIN registries r ON r.id = c.registry_id
  WHERE c.student_id = $1
  AND c.deleted_at IS NULL
  ORDER BY c.date DESC, c.id DESC;

-- UWAGA: service.go robi konwersję strukturalną GetCertificateByIDRow(UpdateCertificateRow),
-- więc oba zapytania muszą zwracać identyczną listę kolumn - te same nazwy, typy
-- i KOLEJNOŚĆ. Nowe kolumny dopisuj na końcu obu list naraz; rozjechana kolejność przy
-- zgodnych typach (renewed_at i duplicate_issued_at są oba timestamptz) byłaby cichą
-- podmianą pól, której kompilator nie wyłapie.
-- name: UpdateCertificate :one
WITH updated AS (
    UPDATE certificates AS c
    SET
        date = sqlc.arg(date),
        student_id = sqlc.arg(student_id),
        coursedatestart = sqlc.arg(course_date_start),
        coursedateend = sqlc.arg(course_date_end),
        student_firstname_snapshot = sqlc.arg(student_firstname_snapshot),
        student_secondname_snapshot = sqlc.arg(student_secondname_snapshot),
        student_lastname_snapshot = sqlc.arg(student_lastname_snapshot),
        student_birthdate_snapshot = sqlc.arg(student_birthdate_snapshot),
        student_birthplace_snapshot = sqlc.arg(student_birthplace_snapshot),
        student_pesel_snapshot = sqlc.arg(student_pesel_snapshot),
        company_name_snapshot = sqlc.arg(company_name_snapshot),
        company_id_snapshot = sqlc.arg(company_id_snapshot)
    WHERE c.id = sqlc.arg(certificate_id)
      AND c.deleted_at IS NULL
    -- RETURNING całego wiersza: główne zapytanie widzi migawkę sprzed UPDATE w CTE, więc
    -- dane zaświadczenia muszą pochodzić z RETURNING, a nie z ponownego odczytu tabeli.
    RETURNING c.*
)
SELECT
    u.id,
    u.date,
    u.student_id,
    u.student_firstname_snapshot AS student_firstname,
    u.student_secondname_snapshot AS student_secondname,
    u.student_lastname_snapshot AS student_lastname,
    u.student_birthdate_snapshot AS student_birthdate,
    u.student_birthplace_snapshot AS student_birthplace,
    u.student_pesel_snapshot AS student_pesel,
    u.company_name_snapshot AS company_name,
    u.coursedatestart AS course_date_start,
    u.coursedateend AS course_date_end,
    r.id AS registry_id,
    r.year AS registry_year,
    r.number::bigint AS registry_number,
    r.course_id AS course_id,
    u.course_name_snapshot AS course_name,
    u.course_symbol_snapshot AS course_symbol,
    u.course_expiry_time_snapshot AS course_expiry_time,
    u.course_program_snapshot::text AS course_program,
    u.cert_front_page_snapshot AS cert_front_page,
    u.language_code,
    tja.id AS journal_attendee_id,
    tj.id AS journal_id,
    tj.title AS journal_title,
    tj.status AS journal_status,
    COALESCE(
        CASE
            WHEN u.coursedateend IS NOT NULL
                AND u.course_expiry_time_snapshot IS NOT NULL
                AND u.course_expiry_time_snapshot ~ '^[0-9]+$'
            THEN TO_CHAR(u.coursedateend + u.course_expiry_time_snapshot::int * 365, 'YYYY-MM-DD')
            ELSE NULL::text
        END,
        ''
    ) AS expiry_date,
    u.verification_code,
    u.revoked_at,
    u.revoke_reason,
    u.duplicate_reason,
    u.duplicate_issued_at,
    u.idempotency_key,
    u.renewed_at,
    u.renewed_by_certificate_id,
    prev.id AS renewal_of_certificate_id
FROM updated u
LEFT JOIN certificates prev ON prev.renewed_by_certificate_id = u.id AND prev.deleted_at IS NULL
LEFT JOIN training_journal_attendees tja ON tja.certificate_id = u.id
LEFT JOIN training_journals tj ON tj.id = tja.journal_id
JOIN registries r ON r.id = u.registry_id;

-- name: SoftDeleteCertificate :one
-- Jedno polecenie, bo usunięcie następcy musi odznaczyć poprzednika niepodzielnie:
-- dokument "przedłużony" zamiennikiem, którego już nie ma, twierdziłby, że sprawa jest
-- załatwiona, i zniknąłby z przypomnień o wygasaniu na zawsze - czyli dokładnie problem,
-- który przedłużanie naprawia (patrz migracja 0028). Polecenia modyfikujące w WITH
-- wykonują się zawsze, niezależnie od tego, czy zapytanie główne czyta ich wynik.
WITH deleted AS (
    UPDATE certificates
    SET
        deleted_at = now(),
        deleted_by_user_id = $2,
        delete_reason = $3
    WHERE certificates.id = $1
      AND deleted_at IS NULL
    RETURNING certificates.id
), predecessor AS (
    UPDATE certificates
    SET renewed_at = NULL,
        renewed_by_certificate_id = NULL,
        renewed_by_user_id = NULL
    -- Gdy "deleted" jest puste (dokument już usunięty), podzapytanie daje NULL
    -- i warunek nie trafia w żaden wiersz.
    WHERE renewed_by_certificate_id = (SELECT deleted.id FROM deleted)
    RETURNING certificates.id
)
SELECT deleted.id FROM deleted;

-- name: CountCertificatesByCourseID :one
SELECT COUNT(*)
FROM certificates c
JOIN registries r ON r.id = c.registry_id
WHERE r.course_id = sqlc.arg(course_id)
  AND (sqlc.narg(date_from)::date IS NULL OR c.date >= sqlc.narg(date_from)::date)
  AND (sqlc.narg(date_to)::date IS NULL OR c.date <= sqlc.narg(date_to)::date)
  AND c.deleted_at IS NULL;

-- name: ListCertificatesByCourseID :many
SELECT
    c.id,
    c.date,
    c.student_firstname_snapshot AS student_firstname,
    c.student_lastname_snapshot AS student_lastname,
    c.company_name_snapshot AS company_name,
    c.course_name_snapshot AS course_name,
    c.course_symbol_snapshot AS course_symbol,
    r.year AS registry_year,
    r.number::bigint AS registry_number,
    c.coursedatestart AS course_date_start,
    c.coursedateend AS course_date_end,
    c.language_code,
    COALESCE(
        CASE
            WHEN c.coursedateend IS NOT NULL
                AND c.course_expiry_time_snapshot IS NOT NULL
                AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
            THEN TO_CHAR(c.coursedateend + c.course_expiry_time_snapshot::int * 365, 'YYYY-MM-DD')
            ELSE NULL::text
        END,
        ''
    ) AS expiry_date,
    c.revoked_at,
    c.duplicate_issued_at,
    c.renewed_at
FROM certificates c
JOIN registries r ON r.id = c.registry_id
WHERE r.course_id = sqlc.arg(course_id)
  AND (sqlc.narg(date_from)::date IS NULL OR c.date >= sqlc.narg(date_from)::date)
  AND (sqlc.narg(date_to)::date IS NULL OR c.date <= sqlc.narg(date_to)::date)
  AND c.deleted_at IS NULL
ORDER BY c.date DESC, c.id DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);

-- name: CountCertificatesByCompanyID :one
  SELECT COUNT(*)
  FROM certificates c
  WHERE c.company_id_snapshot = sqlc.arg(company_id)
    AND (sqlc.narg(date_from)::date IS NULL OR c.date >= sqlc.narg(date_from)::date)
    AND (sqlc.narg(date_to)::date IS NULL OR c.date <= sqlc.narg(date_to)::date)
    AND c.deleted_at IS NULL;

-- name: ListCertificatesByCompanyID :many
  SELECT
      c.id,
      c.date,
      c.student_firstname_snapshot AS student_firstname,
      c.student_lastname_snapshot AS student_lastname,
      c.company_name_snapshot AS company_name,
      c.course_name_snapshot AS course_name,
      c.course_symbol_snapshot AS course_symbol,
      r.year AS registry_year,
      r.number::bigint AS registry_number,
      c.coursedatestart AS course_date_start,
      c.coursedateend AS course_date_end,
      c.language_code,
      COALESCE(
          CASE
              WHEN c.coursedateend IS NOT NULL
                  AND c.course_expiry_time_snapshot IS NOT NULL
                  AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
              THEN TO_CHAR(c.coursedateend + c.course_expiry_time_snapshot::int * 365, 'YYYY-MM-DD')
              ELSE NULL::text
          END,
          ''
      ) AS expiry_date,
      c.revoked_at,
      c.duplicate_issued_at,
      c.renewed_at
  FROM certificates c
  JOIN registries r ON r.id = c.registry_id
  WHERE c.company_id_snapshot = sqlc.arg(company_id)
    AND (sqlc.narg(date_from)::date IS NULL OR c.date >= sqlc.narg(date_from)::date)
    AND (sqlc.narg(date_to)::date IS NULL OR c.date <= sqlc.narg(date_to)::date)
    AND c.deleted_at IS NULL
  ORDER BY c.date DESC, c.id DESC
  LIMIT sqlc.arg(limit_count)
  OFFSET sqlc.arg(offset_count);

-- name: ListExpiringCertificateNotificationCandidates :many
  WITH eligible AS (
    SELECT
      c.id AS certificate_id,
      c.date AS certificate_date,
      c.student_id,
      c.student_firstname_snapshot,
      c.student_lastname_snapshot,
      c.student_pesel_snapshot,
      c.company_name_snapshot,
      comp.id AS company_id,
      comp.name AS company_current_name,
    COALESCE(NULLIF(BTRIM(comp.expiry_notification_email), ''), NULLIF(BTRIM(comp.email), ''))::text AS recipient_email,
      c.course_name_snapshot,
      c.course_symbol_snapshot,
      c.coursedatestart AS course_date_start,
      c.coursedateend AS course_date_end,
      c.language_code,
      r.year AS registry_year,
      r.number::bigint AS registry_number,
    (c.coursedateend + c.course_expiry_time_snapshot::int * 365)::date AS expiry_date
    FROM certificates c
    JOIN companies comp ON comp.id = c.company_id_snapshot
    JOIN registries r ON r.id = c.registry_id
    WHERE c.deleted_at IS NULL
      -- Unieważniony dokument nie wygasa.
      AND c.revoked_at IS NULL
      -- Przedłużony też nie: przypomnienie dotyczy już jego następcy.
      -- Ten sam filtr stoi w dwóch pozostałych zestawieniach wygasających:
      -- dashboard.sql ListExpiringCertificates i CountExpiringCertificates.
      AND c.renewed_at IS NULL
      AND comp.expiry_notifications_enabled = true
      AND c.coursedateend IS NOT NULL
      AND c.course_expiry_time_snapshot IS NOT NULL
      AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
  )
  SELECT *
  FROM eligible
  WHERE recipient_email IS NOT NULL
    AND expiry_date >= sqlc.arg(date_from)::date
    AND expiry_date <= sqlc.arg(date_to)::date
    AND (
      sqlc.narg(after_expiry_date)::date IS NULL
      OR expiry_date > sqlc.narg(after_expiry_date)::date
      OR (
        expiry_date = sqlc.narg(after_expiry_date)::date
        AND certificate_id > sqlc.narg(after_certificate_id)::bigint
      )
    )
  ORDER BY expiry_date ASC, certificate_id ASC
  LIMIT sqlc.arg(limit_count);

-- name: GetCertificateIDByVerificationCode :one
SELECT id
FROM certificates
WHERE verification_code = $1
  AND deleted_at IS NULL;

-- name: LockCertificateForLifecycle :one
-- Blokuje wiersz zaświadczenia na czas unieważnienia, wystawienia duplikatu
-- albo przedłużenia.
-- Stan (unieważnione, przedłużone) sprawdzaj OSOBNYM zapytaniem po uzyskaniu blokady -
-- w READ COMMITTED podzapytanie w tym samym poleceniu widziałoby stan sprzed czekania.
SELECT
    c.id,
    c.coursedatestart AS course_date_start,
    c.coursedateend AS course_date_end,
    r.course_id
FROM certificates c
JOIN registries r ON r.id = c.registry_id
WHERE c.id = $1
  AND c.deleted_at IS NULL
FOR UPDATE OF c;

-- name: GetCertificateLifecycleState :one
-- Cały stan cyklu życia jednym zapytaniem, czytany po uzyskaniu blokady wiersza.
SELECT
    (revoked_at IS NOT NULL)::boolean AS revoked,
    (renewed_at IS NOT NULL)::boolean AS renewed,
    renewed_by_certificate_id
FROM certificates
WHERE id = $1;

-- name: RevokeCertificate :exec
UPDATE certificates
SET revoked_at = now(),
    revoke_reason = sqlc.arg(reason)::text,
    revoked_by_user_id = sqlc.narg(revoked_by_user_id)
WHERE id = sqlc.arg(id);

-- name: MarkCertificateDuplicateIssued :exec
-- Duplikat nie tworzy nowego dokumentu: ten sam wiersz dostaje datę i powód wystawienia
-- wtórnika, a wydruk adnotację "DUPLIKAT". Kolejne wystawienie nadpisuje datę.
UPDATE certificates
SET duplicate_issued_at = now(),
    duplicate_reason = sqlc.arg(reason)::text,
    duplicate_issued_by_user_id = sqlc.narg(duplicate_issued_by_user_id)
WHERE id = sqlc.arg(id);

-- name: MarkCertificateRenewed :exec
-- Przedłużenie odnotowujemy na STARYM dokumencie: wskazuje on następcę i przez to
-- wypada z przypomnień o wygasaniu. Ważności nie traci - to nie jest unieważnienie.
UPDATE certificates
SET renewed_at = now(),
    renewed_by_certificate_id = sqlc.arg(renewed_by_certificate_id),
    renewed_by_user_id = sqlc.narg(renewed_by_user_id)
WHERE id = sqlc.arg(id);

