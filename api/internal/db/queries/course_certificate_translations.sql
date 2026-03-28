-- name: ListCourseCertificateTranslationsByCourseID :many
SELECT
    id,
    course_id,
    language_code,
    course_name,
    course_program::text AS course_program,
    cert_front_page,
    created_at,
    updated_at
FROM course_certificate_translations
WHERE course_id = $1
ORDER BY language_code;

-- name: GetCourseCertificateTranslationByCourseAndLanguage :one
SELECT
    id,
    course_id,
    language_code,
    course_name,
    course_program::text AS course_program,
    cert_front_page,
    created_at,
    updated_at
FROM course_certificate_translations
WHERE course_id = $1
  AND language_code = $2;

-- name: UpsertCourseCertificateTranslation :one
INSERT INTO course_certificate_translations (
    course_id,
    language_code,
    course_name,
    course_program,
    cert_front_page
) VALUES (
    sqlc.arg(course_id),
    sqlc.arg(language_code),
    sqlc.arg(course_name),
    sqlc.arg(course_program),
    sqlc.arg(cert_front_page)
)
ON CONFLICT (course_id, language_code)
DO UPDATE SET
    course_name = EXCLUDED.course_name,
    course_program = EXCLUDED.course_program,
    cert_front_page = EXCLUDED.cert_front_page,
    updated_at = now()
RETURNING
    id,
    course_id,
    language_code,
    course_name,
    course_program::text AS course_program,
    cert_front_page,
    created_at,
    updated_at;

-- name: DeleteCourseCertificateTranslation :execrows
DELETE FROM course_certificate_translations
WHERE course_id = $1
  AND language_code = $2;
