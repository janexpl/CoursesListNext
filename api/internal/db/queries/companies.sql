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
      note,
      expiry_notifications_enabled,
      expiry_notification_email,
      external_id
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
      note = $10,
      expiry_notifications_enabled = $11,
      expiry_notification_email = $12
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
      note,
      expiry_notifications_enabled,
      expiry_notification_email,
      external_id;
    
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
      note,
      expiry_notifications_enabled,
      expiry_notification_email
  ) VALUES (
      $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
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
      note,
      expiry_notifications_enabled,
      expiry_notification_email,
      external_id;

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

-- name: AcquireCompanyExternalIDLock :exec
-- Szereguje równoległe PUT /companies/by-external-id/{externalId} z tym samym identyfikatorem.
SELECT pg_advisory_xact_lock(hashtextextended('company-external-id:' || @external_id::text, 0));

-- name: GetCompanyIDByExternalID :one
SELECT id
FROM companies
WHERE external_id = $1;

-- name: FindCompanyIDByNIP :one
-- Klucz naturalny firmy to NIP (ograniczenie check_unique_nip). excludeID pomija
-- aktualizowaną firmę.
SELECT id
FROM companies
WHERE nip = sqlc.arg(nip)
  AND (sqlc.narg(exclude_id)::bigint IS NULL OR id <> sqlc.narg(exclude_id)::bigint)
LIMIT 1;

-- name: SetCompanyExternalID :exec
UPDATE companies
SET external_id = sqlc.arg(external_id)
WHERE id = sqlc.arg(id);
