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


-- name: ListCoursesDetails :many
-- To samo wyszukiwanie i ta sama kolejność co w ListCourses, ale z pełną
-- treścią kursu: programem szkolenia, szablonem zaświadczenia i tłumaczeniami.
--
-- Tłumaczenia wracają jako gotowa tablica JSON z kluczami odpowiadającymi
-- tagom CourseCertificateTranslationDTO, więc kolumnę wystarczy odpakować
-- json.Unmarshal-em wprost do CourseDetailDTO.CertificateTranslations - bez
-- osobnego zapytania i bez pośrednich struktur.
--
-- Kurs bez tłumaczeń dostaje '[]', nigdy NULL, żeby odpowiedź JSON zawsze
-- miała tablicę.
--
-- Wszystkie te kolumny potrafią być duże, więc po to zapytanie warto sięgać
-- tylko wtedy, gdy treści są faktycznie potrzebne - do samej listy wystarczy
-- ListCourses.
SELECT
    c.id,
    c.mainname,
    c.name,
    c.symbol,
    c.expirytime,
    c.courseprogram,
    c.certfrontpage,
    COALESCE(t.translations, '[]'::json)::json AS certificate_translations
FROM courses c
LEFT JOIN LATERAL (
    SELECT json_agg(
        json_build_object(
            'languageCode', ct.language_code,
            'courseName', ct.course_name,
            -- ::text, bo CourseProgram w DTO jest stringiem niosącym JSON,
            -- tak samo jak w ścieżce pojedynczego kursu.
            'courseProgram', ct.course_program::text,
            'certFrontPage', ct.cert_front_page
        )
        ORDER BY ct.language_code
    ) AS translations
    FROM course_certificate_translations ct
    WHERE ct.course_id = c.id
) t ON TRUE
WHERE
    (
        sqlc.narg(search)::text IS NULL
        OR COALESCE(c.mainname, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(c.name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
        OR COALESCE(c.symbol, '') ILIKE '%' || sqlc.narg(search)::text || '%'
    )
ORDER BY
    CASE
        WHEN sqlc.narg(search)::text IS NULL THEN 5
        WHEN LOWER(COALESCE(c.symbol, '')) = LOWER(sqlc.narg(search)::text) THEN 0
        WHEN LOWER(COALESCE(c.symbol, '')) LIKE LOWER(sqlc.narg(search)::text) || '%' THEN 1
        WHEN LOWER(COALESCE(c.name, '')) LIKE LOWER(sqlc.narg(search)::text) || '%' THEN 2
        WHEN LOWER(COALESCE(c.mainname, '')) LIKE LOWER(sqlc.narg(search)::text) || '%' THEN 3
        WHEN LOWER(COALESCE(c.symbol, '')) LIKE '%' || LOWER(sqlc.narg(search)::text) || '%' THEN 4
        ELSE 5
    END,
    c.symbol,
    c.name
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
