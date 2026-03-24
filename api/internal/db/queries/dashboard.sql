-- name: GetDashboardStats :one
SELECT
    (SELECT COUNT(*) FROM students) AS total_students,
    (SELECT COUNT(*) FROM companies) AS total_companies,
    (SELECT COUNT(*) FROM certificates WHERE deleted_at IS NULL) AS total_certificates;


-- name: ListExpiringCertificates :many
SELECT
    c.id,
    TO_CHAR(c.coursedateend + cr.expirytime::int * 365, 'YYYY-MM-DD') AS expiry_date,
    s.firstname,
    s.lastname,
    comp.name AS company_name,
    r.year,
    r.number,
    cr.name AS course_name,
    cr.symbol AS course_symbol
FROM certificates c
JOIN students s ON c.student_id = s.id
JOIN companies comp ON s.company_id = comp.id
JOIN registries r ON c.registry_id = r.id
JOIN courses cr ON r.course_id = cr.id
WHERE c.coursedateend IS NOT NULL
  AND (c.coursedateend + cr.expirytime::int * 365) >= CURRENT_DATE
  AND (c.coursedateend + cr.expirytime::int * 365) < CURRENT_DATE + INTERVAL '30 days'
  AND c.deleted_at IS NULL
ORDER BY expiry_date ASC
LIMIT 50;

-- name: CountExpiringCertificates :one
SELECT COUNT(*) FROM certificates c
JOIN registries r ON c.registry_id = r.id
JOIN courses cr ON r.course_id = cr.id
WHERE c.coursedateend IS NOT NULL
  AND (c.coursedateend + cr.expirytime::int * 365) >= CURRENT_DATE
  AND (c.coursedateend + cr.expirytime::int * 365) < CURRENT_DATE + INTERVAL '30 days'
  AND c.deleted_at IS NULL;