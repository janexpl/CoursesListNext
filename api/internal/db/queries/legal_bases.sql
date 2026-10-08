-- name: ListLegalBases :many
-- Lista do wyboru w ustawieniach kursu i w zakładce administratora. course_count mówi,
-- ilu kursów dotknie zmiana treści.
SELECT
    lb.id,
    lb.name,
    lb.content,
    lb.updated_at,
    (SELECT count(*) FROM courses c WHERE c.legal_basis_id = lb.id)::bigint AS course_count
FROM legal_bases lb
ORDER BY lower(lb.name), lb.id;

-- name: GetLegalBasisByID :one
SELECT id, name, content, created_at, updated_at
FROM legal_bases
WHERE id = $1;

-- name: ListCoursesByLegalBasis :many
SELECT id, symbol, name
FROM courses
WHERE legal_basis_id = $1
ORDER BY symbol, id;

-- name: CreateLegalBasis :one
INSERT INTO legal_bases (name, content)
VALUES (sqlc.arg(name), sqlc.arg(content))
RETURNING id, name, content, created_at, updated_at;

-- name: UpdateLegalBasis :one
UPDATE legal_bases
SET name = sqlc.arg(name),
    content = sqlc.arg(content)
WHERE id = sqlc.arg(id)
RETURNING id, name, content, created_at, updated_at;

-- name: DeleteLegalBasis :execrows
DELETE FROM legal_bases
WHERE id = $1;
