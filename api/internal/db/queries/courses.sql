-- name: ListCourses :many
SELECT
    id,
    mainname,
    name,
    symbol,
    expirytime
FROM courses
WHERE
    (
        sqlc.narg(search)::text IS NULL
        OR COALESCE(mainname, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(symbol, '') ILIKE '%' || sqlc.narg(search)::text || '%'
    )
ORDER BY
    CASE
        WHEN sqlc.narg(search)::text IS NULL THEN 5
        WHEN LOWER(COALESCE(symbol, '')) = LOWER(sqlc.narg(search)::text) THEN 0
        WHEN LOWER(COALESCE(symbol, '')) LIKE LOWER(sqlc.narg(search)::text) || '%' THEN 1
        WHEN LOWER(COALESCE(name, '')) LIKE LOWER(sqlc.narg(search)::text) || '%' THEN 2
        WHEN LOWER(COALESCE(mainname, '')) LIKE LOWER(sqlc.narg(search)::text) || '%' THEN 3
        WHEN LOWER(COALESCE(symbol, '')) LIKE '%' || LOWER(sqlc.narg(search)::text) || '%' THEN 4
        ELSE 5
    END,
    symbol,
    name
LIMIT sqlc.arg(limit_count);


-- name: GetCourseByID :one
SELECT id, mainname, name, symbol, expirytime, courseprogram, certfrontpage FROM courses
WHERE id = $1;


-- name: UpdateCourse :one
  UPDATE courses
  SET
      mainname = $2,
      name = $3,
      symbol = $4,
      expirytime = $5,
      courseprogram = $6,
      certfrontpage = $7
  WHERE id = $1
  RETURNING id, mainname, name, symbol, expirytime, courseprogram, certfrontpage;

-- name: CreateCourse :one
  INSERT INTO courses (
      mainname,
      name,
      symbol,
      expirytime,
      courseprogram,
      certfrontpage
  ) VALUES (
      $1, $2, $3, $4, $5, $6
  )
  RETURNING
      id,
      mainname,
      name,
      symbol,
      expirytime,
      courseprogram,
      certfrontpage;
