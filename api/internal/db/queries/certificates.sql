-- name: ListCertificates :many
  SELECT
      c.id,
      c.date,
      s.firstname AS student_firstname,
      s.lastname AS student_lastname,
      comp.name AS company_name,
      cr.name AS course_name,
      cr.symbol AS course_symbol,
      r.year AS registry_year,
      r.number::bigint AS registry_number,
      c.coursedatestart AS course_date_start,
      c.coursedateend AS course_date_end,
      COALESCE(CASE
          WHEN c.coursedateend IS NOT NULL
              AND cr.expirytime IS NOT NULL
              AND cr.expirytime ~ '^[0-9]+$'
          THEN TO_CHAR(c.coursedateend + cr.expirytime::int * 365, 'YYYY-MM-DD')
          ELSE NULL::text
      END, '') AS expiry_date
  FROM certificates c
  JOIN students s ON s.id = c.student_id
  LEFT JOIN companies comp ON comp.id = s.company_id
  JOIN registries r ON r.id = c.registry_id
  JOIN courses cr ON cr.id = r.course_id
  WHERE
      (
          sqlc.narg(search)::text IS NULL
          OR COALESCE(s.firstname, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(s.lastname, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(comp.name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(cr.name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(cr.symbol, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR (
              r.number::bigint::text || '/' || cr.symbol || '/' || r.year::text
          ) ILIKE '%' || sqlc.narg(search)::text || '%'
      )
      AND c.deleted_at IS NULL
  ORDER BY c.date DESC, c.id DESC
  LIMIT sqlc.arg(limit_count);


-- name: GetCertificateByID :one
SELECT
    c.id,
    c.date,
    s.id AS student_id,
    s.firstname AS student_firstname,
    s.secondname AS student_secondname,
    s.lastname AS student_lastname,
    s.birthdate AS student_birthdate,
    s.birthplace AS student_birthplace,
    s.pesel AS student_pesel,
    comp.id AS company_id,
    comp.name AS company_name,
    c.coursedatestart AS course_date_start,
    c.coursedateend AS course_date_end,
    r.id AS registry_id,
    r.year AS registry_year,
    r.number::bigint AS registry_number,
    cr.id AS course_id,
    cr.mainname AS course_mainname,
    cr.name AS course_name,
    cr.symbol AS course_symbol,
    cr.expirytime AS course_expiry_time,
    cr.courseprogram::text AS course_program,
    cr.certfrontpage AS cert_front_page,
    tja.id AS journal_attendee_id,
    tj.id AS journal_id,
    tj.title AS journal_title,
    tj.status AS journal_status,
    COALESCE(CASE
        WHEN c.coursedateend IS NOT NULL
            AND cr.expirytime IS NOT NULL
            AND cr.expirytime ~ '^[0-9]+$'
        THEN TO_CHAR(c.coursedateend + cr.expirytime::int * 365, 'YYYY-MM-DD')
        ELSE NULL::text
    END, '') AS expiry_date
FROM certificates c
JOIN students s ON s.id = c.student_id
LEFT JOIN companies comp ON comp.id = s.company_id
LEFT JOIN training_journal_attendees tja ON tja.certificate_id = c.id
LEFT JOIN training_journals tj ON tj.id = tja.journal_id
JOIN registries r ON r.id = c.registry_id
JOIN courses cr ON cr.id = r.course_id
WHERE c.id = $1 AND c.deleted_at IS NULL;

-- name: CreateCertificate :one
INSERT INTO certificates (
    date,
    student_id,
    coursedatestart,
    coursedateend,
    registry_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id;
 
-- name: ListCertificatesByStudentID :many
  SELECT
      c.id,
      c.date,
      cr.name AS course_name,
      cr.symbol AS course_symbol,
      r.year AS registry_year,
      r.number::bigint AS registry_number,
      c.coursedatestart AS course_date_start,
      c.coursedateend AS course_date_end,
      COALESCE(CASE
          WHEN c.coursedateend IS NOT NULL
              AND cr.expirytime IS NOT NULL
              AND cr.expirytime ~ '^[0-9]+$'
          THEN TO_CHAR(c.coursedateend + cr.expirytime::int * 365, 'YYYY-MM-DD')
          ELSE NULL::text
      END, '') AS expiry_date
  FROM certificates c
  JOIN registries r ON r.id = c.registry_id
  JOIN courses cr ON cr.id = r.course_id
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
          coursedateend = sqlc.arg(course_date_end)
      WHERE c.id = sqlc.arg(certificate_id)
      AND c.deleted_at IS NULL
      RETURNING c.id
  )
  SELECT
      c.id,
      c.date,
      s.id AS student_id,
      s.firstname AS student_firstname,
      s.secondname AS student_secondname,
      s.lastname AS student_lastname,
      s.birthdate AS student_birthdate,
      s.birthplace AS student_birthplace,
      s.pesel AS student_pesel,
      comp.id AS company_id,
      comp.name AS company_name,
      c.coursedatestart AS course_date_start,
      c.coursedateend AS course_date_end,
      r.id AS registry_id,
      r.year AS registry_year,
      r.number::bigint AS registry_number,
      cr.id AS course_id,
      cr.mainname AS course_mainname,
      cr.name AS course_name,
      cr.symbol AS course_symbol,
      cr.expirytime AS course_expiry_time,
      cr.courseprogram::text AS course_program,
      cr.certfrontpage AS cert_front_page,
      tja.id AS journal_attendee_id,
      tj.id AS journal_id,
      tj.title AS journal_title,
      tj.status AS journal_status,
      COALESCE(CASE
          WHEN c.coursedateend IS NOT NULL
              AND cr.expirytime IS NOT NULL
              AND cr.expirytime ~ '^[0-9]+$'
          THEN TO_CHAR(c.coursedateend + cr.expirytime::int * 365, 'YYYY-MM-DD')
          ELSE NULL::text
      END, '') AS expiry_date
  FROM updated u
  JOIN certificates c ON c.id = u.id
  JOIN students s ON s.id = c.student_id
  LEFT JOIN training_journal_attendees tja ON tja.certificate_id = c.id
  LEFT JOIN training_journals tj ON tj.id = tja.journal_id
  LEFT JOIN companies comp ON comp.id = s.company_id
  JOIN registries r ON r.id = c.registry_id
  JOIN courses cr ON cr.id = r.course_id;

-- name: SoftDeleteCertificate :one
  UPDATE certificates
  SET
      deleted_at = now(),
      deleted_by_user_id = $2,
      delete_reason = $3
  WHERE id = $1
    AND deleted_at IS NULL
  RETURNING id;

