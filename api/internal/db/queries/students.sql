-- name: ListStudents :many
  SELECT
      s.id,
      s.firstname,
      s.lastname,
      s.secondname,
      s.birthdate,
      s.birthplace,
      s.pesel,
      c.id AS company_id,
      c.name AS company_name
  FROM students s
  LEFT JOIN companies c ON c.id = s.company_id
  WHERE
      (
          sqlc.narg(search)::text IS NULL
          OR COALESCE(s.firstname, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(s.lastname, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(s.pesel, '') ILIKE '%' || sqlc.narg(search)::text || '%'
      )
      AND (
          sqlc.narg(company_id)::bigint IS NULL
          OR s.company_id = sqlc.narg(company_id)::bigint
      )
  ORDER BY s.lastname, s.firstname
  LIMIT sqlc.arg(limit_count);

-- name: GetStudentByID :one
  SELECT
      s.id,
      s.firstname,
      s.lastname,
      s.secondname,
      s.birthdate,
      s.birthplace,
      s.pesel,
      s.addressstreet,
      s.addresscity,
      s.addresszip,
      s.telephoneno,
      c.id AS company_id,
      c.name AS company_name
  FROM students s
  LEFT JOIN companies c ON c.id = s.company_id
  WHERE s.id = $1;

-- name: ListStudentsByCompanyID :many
  SELECT
      s.id,
      s.firstname,
      s.lastname,
      s.secondname,
      s.birthdate,
      s.birthplace,
      s.pesel
  FROM students s
  WHERE s.company_id = $1
  ORDER BY s.lastname, s.firstname;


-- name: UpdateStudent :one
  WITH updated AS (
      UPDATE students AS s
      SET
          firstname = sqlc.arg(firstname),
          lastname = sqlc.arg(lastname),
          secondname = sqlc.arg(secondname),
          birthdate = sqlc.arg(birthdate),
          birthplace = sqlc.arg(birthplace),
          pesel = sqlc.arg(pesel),
          addressstreet = sqlc.arg(addressstreet),
          addresscity = sqlc.arg(addresscity),
          addresszip = sqlc.arg(addresszip),
          telephoneno = sqlc.arg(telephoneno),
          company_id = sqlc.arg(company_id)
      WHERE s.id = sqlc.arg(student_id)
       RETURNING
          s.id,
          s.firstname,
          s.lastname,
          s.secondname,
          s.birthdate,
          s.birthplace,
          s.pesel,
          s.addressstreet,
          s.addresscity,
          s.addresszip,
          s.telephoneno,
          s.company_id

  )
  SELECT
      u.id,
      u.firstname,
      u.lastname,
      u.secondname,
      u.birthdate,
      u.birthplace,
      u.pesel,
      u.addressstreet,
      u.addresscity,
      u.addresszip,
      u.telephoneno,
      c.id AS company_id,
      c.name AS company_name
  FROM updated u
  LEFT JOIN companies c ON c.id = u.company_id;

-- name: CreateStudent :one
  WITH inserted AS (
      INSERT INTO students (
          firstname,
          lastname,
          secondname,
          birthdate,
          birthplace,
          pesel,
          addressstreet,
          addresscity,
          addresszip,
          telephoneno,
          company_id
      ) VALUES (
          sqlc.arg(firstname),
          sqlc.arg(lastname),
          sqlc.arg(secondname),
          sqlc.arg(birthdate),
          sqlc.arg(birthplace),
          sqlc.arg(pesel),
          sqlc.arg(addressstreet),
          sqlc.arg(addresscity),
          sqlc.arg(addresszip),
          sqlc.arg(telephoneno),
          sqlc.arg(company_id)
      )
      RETURNING
          id,
          firstname,
          lastname,
          secondname,
          birthdate,
          birthplace,
          pesel,
          addressstreet,
          addresscity,
          addresszip,
          telephoneno,
          company_id
  )
  SELECT
      s.id,
      s.firstname,
      s.lastname,
      s.secondname,
      s.birthdate,
      s.birthplace,
      s.pesel,
      s.addressstreet,
      s.addresscity,
      s.addresszip,
      s.telephoneno,
      c.id AS company_id,
      c.name AS company_name
  FROM inserted s
  LEFT JOIN companies c ON c.id = s.company_id;


