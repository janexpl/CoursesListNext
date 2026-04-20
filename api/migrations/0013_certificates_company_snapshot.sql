  ALTER TABLE certificates
      ADD COLUMN company_id_snapshot bigint REFERENCES companies(id) ON DELETE RESTRICT;

  CREATE INDEX certificates_company_id_snapshot_idx
      ON certificates (company_id_snapshot)
      WHERE company_id_snapshot IS NOT NULL;

  -- Bezpieczny backfill tylko tam, gdzie aktualna firma kursanta zgadza się z historyczną nazwą firmy.
  WITH matched AS (
      SELECT
          cert.id,
          s.company_id
      FROM certificates cert
      JOIN students s ON s.id = cert.student_id
      JOIN companies c ON c.id = s.company_id
      WHERE cert.company_id_snapshot IS NULL
        AND s.company_id IS NOT NULL
        AND cert.company_name_snapshot IS NOT DISTINCT FROM c.name
  )
  UPDATE certificates cert
  SET company_id_snapshot = matched.company_id
  FROM matched
  WHERE cert.id = matched.id;

  -- Porządki przed dodaniem FK dla aktywnego powiązania kursanta z firmą.
  UPDATE students s
  SET company_id = NULL
  WHERE company_id IS NOT NULL
    AND NOT EXISTS (
        SELECT 1
        FROM companies c
        WHERE c.id = s.company_id
    );

  ALTER TABLE students
      ADD CONSTRAINT students_company_id_fkey
      FOREIGN KEY (company_id) REFERENCES companies(id) ON DELETE SET NULL;

  CREATE INDEX students_company_id_idx
      ON students (company_id)
      WHERE company_id IS NOT NULL;
