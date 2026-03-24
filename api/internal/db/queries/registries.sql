-- name: GetNextRegistryNumber :one
SELECT COALESCE(MAX(number), 0) + 1 AS next_number
FROM registries
WHERE course_id = $1 AND year = $2;

-- name: CreateRegistry :one
INSERT INTO registries (
    course_id,
    year,
    number
) VALUES (
    $1,
    $2,
    $3
)
RETURNING id;

-- name: ListRegistryDatesForCourseYear :many
SELECT
    c.date AS certificate_date,
    r.number AS registry_number
FROM certificates c
JOIN registries r ON r.id = c.registry_id
WHERE r.course_id = $1
  AND r.year = $2
  AND c.deleted_at IS NULL
ORDER BY r.number;
