-- name: ListCompanies :many
SELECT
      id,
      name,
      city,
      nip,
      contactperson,
      telephoneno
  FROM companies
  WHERE
      (
          sqlc.narg(search)::text IS NULL
          OR COALESCE(name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(city, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(nip, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(contactperson, '') ILIKE '%' || sqlc.narg(search)::text || '%'
      )
  ORDER BY name, city
  LIMIT sqlc.arg(limit_count);

-- name: GetCompanyByID :one
  SELECT
      id,
      name,
      street,
      city,
      zipcode,
      nip,
      email,
      contactperson,
      telephoneno,
      note
  FROM companies
  WHERE id = $1;

-- name: UpdateCompany :one
  UPDATE companies
  SET
      name = $2,
      street = $3,  
      city = $4,
      zipcode = $5,
      nip = $6,
      email = $7,
      contactperson = $8,
      telephoneno = $9,
      note = $10
  WHERE id = $1
  RETURNING
      id,
      name,
      street,
      city,
      zipcode,
      nip,
      email,
      contactperson,
      telephoneno,
      note;
    
-- name: CreateCompany :one
  INSERT INTO companies (
      name,
      street,
      city,
      zipcode,
      nip,
      email,
      contactperson,
      telephoneno,
      note
  ) VALUES (
      $1, $2, $3, $4, $5, $6, $7, $8, $9
  )
  RETURNING
      id,
      name,
      street,
      city,
      zipcode,
      nip,
      email,
      contactperson,
      telephoneno,
      note;

-- name: CompanyHasCertificatesHistory :one
  SELECT EXISTS (
      SELECT 1
      FROM certificates
      WHERE company_id_snapshot = $1
  );

  -- name: DeleteCompany :one
  DELETE FROM companies
  WHERE id = $1
  RETURNING id;

      




