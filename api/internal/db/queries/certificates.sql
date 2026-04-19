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
    ) AS expiry_date
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
    AND c.deleted_at IS NULL
ORDER BY c.date DESC, c.id DESC
LIMIT sqlc.arg(limit_count);

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
    ) AS expiry_date
FROM certificates c
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
    sqlc.arg(course_name_snapshot),
    sqlc.arg(course_symbol_snapshot),
    sqlc.arg(course_expiry_time_snapshot),
    sqlc.arg(course_program_snapshot),
    sqlc.arg(cert_front_page_snapshot)
)
RETURNING id;

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
      END, '') AS expiry_date
  FROM certificates c
  JOIN registries r ON r.id = c.registry_id
  WHERE c.student_id = $1
  AND c.deleted_at IS NULL
  ORDER BY c.date DESC, c.id DESC;

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
        company_name_snapshot = sqlc.arg(company_name_snapshot)
    WHERE c.id = sqlc.arg(certificate_id)
      AND c.deleted_at IS NULL
    RETURNING c.id
)
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
    ) AS expiry_date
FROM updated u
JOIN certificates c ON c.id = u.id
LEFT JOIN training_journal_attendees tja ON tja.certificate_id = c.id
LEFT JOIN training_journals tj ON tj.id = tja.journal_id
JOIN registries r ON r.id = c.registry_id;

-- name: SoftDeleteCertificate :one
  UPDATE certificates
  SET
      deleted_at = now(),
      deleted_by_user_id = $2,
      delete_reason = $3
  WHERE id = $1
    AND deleted_at IS NULL
  RETURNING id;

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
    ) AS expiry_date
FROM certificates c
JOIN registries r ON r.id = c.registry_id
WHERE r.course_id = sqlc.arg(course_id)
  AND (sqlc.narg(date_from)::date IS NULL OR c.date >= sqlc.narg(date_from)::date)
  AND (sqlc.narg(date_to)::date IS NULL OR c.date <= sqlc.narg(date_to)::date)
  AND c.deleted_at IS NULL
ORDER BY c.date DESC, c.id DESC
LIMIT sqlc.arg(limit_count)
OFFSET sqlc.arg(offset_count);