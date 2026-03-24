-- name: ListUsers :many
  SELECT
      id,
      email,
      firstname,
      lastname,
      role
  FROM users
  ORDER BY lastname, firstname, email;

-- name: CreateUser :one
  INSERT INTO users (
      email,
      password,
      firstname,
      lastname,
      role
  ) VALUES (
      $1, $2, $3, $4, $5
  )
  RETURNING
      id,
      email,
      firstname,
      lastname,
      role;

-- name: DeleteUser :execrows
  DELETE FROM users
  WHERE id = $1;

-- name: CountAdminUsers :one
  SELECT COUNT(*)
  FROM users
  WHERE role = $1;

-- name: UpdateUser :one
  UPDATE users
  SET
      email = $2,
      firstname = $3,
      lastname = $4,
      role = $5
  WHERE id = $1
  RETURNING
      id,
      email,
      firstname,
      lastname,
      role;

-- name: UpdateUserPassword :exec
  UPDATE users
  SET password = $2
  WHERE id = $1;
