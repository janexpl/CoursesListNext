-- name: ListCertificatePrintAssetsMeta :many
  SELECT
      id,
      kind,
      file_name,
      content_type,
      file_size,
      print_width_mm,
      uploaded_by_user_id,
      created_at,
      updated_at
  FROM certificate_print_assets
  ORDER BY kind;

-- ListCertificatePrintAssetFiles zwraca bajty wszystkich nadruków jednym zapytaniem -
-- wydruk potrzebuje ich naraz, a rodzajów jest najwyżej trzy.
-- name: ListCertificatePrintAssetFiles :many
  SELECT
      kind,
      content_type,
      file_data,
      print_width_mm
  FROM certificate_print_assets
  ORDER BY kind;

-- name: GetCertificatePrintAssetFile :one
  SELECT
      kind,
      file_name,
      content_type,
      file_data,
      print_width_mm,
      updated_at
  FROM certificate_print_assets
  WHERE kind = $1;

-- name: UpsertCertificatePrintAsset :one
  INSERT INTO certificate_print_assets (
      kind,
      file_name,
      content_type,
      file_size,
      file_data,
      print_width_mm,
      uploaded_by_user_id
  ) VALUES (
      $1, $2, $3, $4, $5, $6, $7
  )
  ON CONFLICT (kind) DO UPDATE
  SET
      file_name = EXCLUDED.file_name,
      content_type = EXCLUDED.content_type,
      file_size = EXCLUDED.file_size,
      file_data = EXCLUDED.file_data,
      print_width_mm = EXCLUDED.print_width_mm,
      uploaded_by_user_id = EXCLUDED.uploaded_by_user_id,
      updated_at = now()
  RETURNING
      id,
      kind,
      file_name,
      content_type,
      file_size,
      print_width_mm,
      uploaded_by_user_id,
      created_at,
      updated_at;

-- name: DeleteCertificatePrintAsset :execrows
  DELETE FROM certificate_print_assets
  WHERE kind = $1;
