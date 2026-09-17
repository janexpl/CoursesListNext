-- name: GetDashboardStats :one
SELECT
    (SELECT COUNT(*) FROM students) AS total_students,
    (SELECT COUNT(*) FROM companies) AS total_companies,
    (SELECT COUNT(*) FROM certificates WHERE deleted_at IS NULL) AS total_certificates;


-- name: ListExpiringCertificates :many
-- Ważność liczona tak samo jak expiry_date w ListCertificates: z okresu ważności
-- zapisanego w zaświadczeniu w chwili wystawienia, a nie z aktualnego okresu kursu.
-- Dane kursanta, firmy i kursu też pochodzą z kopii w zaświadczeniu, więc zaświadczenie
-- kursanta bez firmy jest na liście (z pustą nazwą firmy), tak jak w ogólnej liście.
-- CASE gwarantuje, że rzutowanie na int nie wykona się dla nieliczbowego okresu.
WITH expiring AS (
    SELECT
        c.id,
        CASE
            WHEN c.coursedateend IS NOT NULL AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
            THEN c.coursedateend + c.course_expiry_time_snapshot::int * 365
        END AS expiry,
        c.student_firstname_snapshot,
        c.student_lastname_snapshot,
        c.company_name_snapshot,
        c.course_name_snapshot,
        c.course_symbol_snapshot,
        c.registry_id
    FROM certificates c
    WHERE c.deleted_at IS NULL
      AND c.revoked_at IS NULL
)
SELECT
    e.id,
    TO_CHAR(e.expiry, 'YYYY-MM-DD') AS expiry_date,
    e.student_firstname_snapshot AS firstname,
    e.student_lastname_snapshot AS lastname,
    COALESCE(e.company_name_snapshot, '') AS company_name,
    r.year,
    r.number,
    e.course_name_snapshot AS course_name,
    e.course_symbol_snapshot AS course_symbol
FROM expiring e
JOIN registries r ON r.id = e.registry_id
WHERE e.expiry >= CURRENT_DATE
  AND e.expiry < CURRENT_DATE + 30
ORDER BY e.expiry ASC, e.id ASC
LIMIT 50;

-- name: CountExpiringCertificates :one
-- Ten sam zbiór co ListExpiringCertificates, bez limitu. Wcześniej licznik i lista
-- różniły się warunkami (lista pomijała kursantów bez firmy).
SELECT COUNT(*)
FROM certificates c
WHERE c.deleted_at IS NULL
  AND c.revoked_at IS NULL
  AND (
      CASE
          WHEN c.coursedateend IS NOT NULL AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
          THEN c.coursedateend + c.course_expiry_time_snapshot::int * 365
      END
  ) >= CURRENT_DATE
  AND (
      CASE
          WHEN c.coursedateend IS NOT NULL AND c.course_expiry_time_snapshot ~ '^[0-9]+$'
          THEN c.coursedateend + c.course_expiry_time_snapshot::int * 365
      END
  ) < CURRENT_DATE + 30;
