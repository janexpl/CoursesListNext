-- name: ListJournals :many
  SELECT
      j.id,
      j.title,
      j.course_symbol,
      j.organizer_name,
      j.location,
      j.form_of_training,
      j.date_start,
      j.date_end,
      j.total_hours,
      j.status,
      j.created_at,
      c.id AS course_id,
      c.name AS course_name,
      comp.id AS company_id,
      comp.name AS company_name,
      (
          SELECT COUNT(*)::bigint
          FROM training_journal_attendees a
          WHERE a.journal_id = j.id
      ) AS attendees_count,
      (
          SELECT COUNT(*)::bigint
          FROM training_journal_sessions s
          WHERE s.journal_id = j.id
      ) AS sessions_count
  FROM training_journals j
  JOIN courses c ON c.id = j.course_id
  LEFT JOIN companies comp ON comp.id = j.company_id
  WHERE
      (
          sqlc.narg(search)::text IS NULL
          OR COALESCE(j.title, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(j.course_symbol, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(c.name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(comp.name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(j.location, '') ILIKE '%' || sqlc.narg(search)::text || '%'
          OR COALESCE(j.organizer_name, '') ILIKE '%' || sqlc.narg(search)::text || '%'
      )
      AND (
          sqlc.narg(course_id)::bigint IS NULL
          OR j.course_id = sqlc.narg(course_id)::bigint
      )
      AND (
          sqlc.narg(company_id)::bigint IS NULL
          OR j.company_id = sqlc.narg(company_id)::bigint
      )
      AND (
          sqlc.narg(status)::text IS NULL
          OR j.status = sqlc.narg(status)::text
      )
      AND (
          sqlc.narg(date_from)::date IS NULL
          OR j.date_start >= sqlc.narg(date_from)::date
      )
      AND (
          sqlc.narg(date_to)::date IS NULL
          OR j.date_end <= sqlc.narg(date_to)::date
      )
  ORDER BY j.date_start DESC, j.id DESC
  LIMIT sqlc.arg(limit_count);


-- name: CreateJournal :one
  WITH selected_course AS (
      SELECT
          c.id,
          c.symbol,
          COALESCE(c.courseprogram::jsonb, '[]'::jsonb) AS courseprogram,
          COALESCE(
              (
                  SELECT SUM(
                      COALESCE(NULLIF(entry->>'TheoryTime', '')::numeric, 0)
                      + COALESCE(NULLIF(entry->>'PracticeTime', '')::numeric, 0)
                  )
                  FROM jsonb_array_elements(COALESCE(c.courseprogram::jsonb, '[]'::jsonb)) entry
              ),
              0::numeric
          ) AS total_hours
      FROM courses c
      WHERE c.id = sqlc.arg(course_id)
  ),
  inserted AS (
      INSERT INTO training_journals (
          course_id,
          company_id,
          title,
          course_symbol,
          organizer_name,
          organizer_address,
          location,
          form_of_training,
          legal_basis,
          date_start,
          date_end,
          total_hours,
          notes,
          created_by_user_id
      )
      SELECT
          sqlc.arg(course_id),
          sqlc.arg(company_id),
          sqlc.arg(title),
          sc.symbol,
          sqlc.arg(organizer_name),
          sqlc.arg(organizer_address),
          sqlc.arg(location),
          sqlc.arg(form_of_training),
          sqlc.arg(legal_basis),
          sqlc.arg(date_start),
          sqlc.arg(date_end),
          sc.total_hours,
          sqlc.arg(notes),
          sqlc.arg(created_by_user_id)
      FROM selected_course sc
      RETURNING
          id,
          course_id,
          company_id,
          title,
          course_symbol,
          organizer_name,
          organizer_address,
          location,
          form_of_training,
          legal_basis,
          date_start,
          date_end,
          total_hours,
          notes,
          status,
          created_by_user_id,
          created_at,
          updated_at,
          closed_at
  ),
  inserted_sessions AS (
      INSERT INTO training_journal_sessions (
          journal_id,
          session_date,
          hours,
          topic,
          trainer_name,
          sort_order
      )
      SELECT
          i.id,
          i.date_start,
          COALESCE(NULLIF(entry.item->>'TheoryTime', '')::numeric, 0)
              + COALESCE(NULLIF(entry.item->>'PracticeTime', '')::numeric, 0),
          COALESCE(NULLIF(TRIM(entry.item->>'Subject'), ''), 'Temat ' || entry.ordinality::text),
          i.organizer_name,
          entry.ordinality::integer
      FROM inserted i
      CROSS JOIN selected_course sc
      CROSS JOIN LATERAL jsonb_array_elements(COALESCE(sc.courseprogram::jsonb, '[]'::jsonb))
          WITH ORDINALITY AS entry(item, ordinality)
      WHERE
          TRIM(COALESCE(entry.item->>'Subject', '')) <> ''
          OR COALESCE(NULLIF(entry.item->>'TheoryTime', '')::numeric, 0) > 0
          OR COALESCE(NULLIF(entry.item->>'PracticeTime', '')::numeric, 0) > 0
  )
  SELECT
      i.id,
      i.course_id,
      c.name AS course_name,
      i.company_id,
      comp.name AS company_name,
      i.title,
      i.course_symbol,
      i.organizer_name,
      i.organizer_address,
      i.location,
      i.form_of_training,
      i.legal_basis,
      i.date_start,
      i.date_end,
      i.total_hours,
      i.notes,
      i.status,
      i.created_by_user_id,
      i.created_at,
      i.updated_at,
      i.closed_at,
      0::bigint AS attendees_count,
      0::bigint AS sessions_count
  FROM inserted i
  JOIN courses c ON c.id = i.course_id
  LEFT JOIN companies comp ON comp.id = i.company_id;

-- name: GetJournalByID :one
  SELECT
      j.id,
      j.course_id,
      c.name AS course_name,
      j.company_id,
      comp.name AS company_name,
      j.title,
      j.course_symbol,
      j.organizer_name,
      j.organizer_address,
      j.location,
      j.form_of_training,
      j.legal_basis,
      j.date_start,
      j.date_end,
      j.total_hours,
      j.notes,
      j.status,
      j.created_by_user_id,
      j.created_at,
      j.updated_at,
      j.closed_at,
      (
          SELECT COUNT(*)::bigint
          FROM training_journal_attendees a
          WHERE a.journal_id = j.id
      ) AS attendees_count,
      (
          SELECT COUNT(*)::bigint
          FROM training_journal_sessions s
          WHERE s.journal_id = j.id
      ) AS sessions_count
  FROM training_journals j
  JOIN courses c ON c.id = j.course_id
  LEFT JOIN companies comp ON comp.id = j.company_id
  WHERE j.id = $1;

-- name: DeleteJournal :execrows
  DELETE FROM training_journals
  WHERE id = $1;

-- name: CloseJournal :execrows
  UPDATE training_journals
  SET
      status = 'closed',
      closed_at = COALESCE(closed_at, now()),
      updated_at = now()
  WHERE id = $1
    AND status <> 'closed';

-- name: ListJournalAttendees :many
  SELECT
      a.id,
      a.journal_id,
      a.student_id,
      a.certificate_id,
      a.full_name_snapshot,
      a.birthdate_snapshot,
      a.company_name_snapshot,
      a.sort_order,
      a.created_at,
      c.date AS certificate_date,
      r.year AS certificate_registry_year,
      COALESCE(r.number::bigint, 0::bigint) AS certificate_registry_number,
      cr.symbol AS certificate_course_symbol
  FROM training_journal_attendees a
  LEFT JOIN certificates c ON c.id = a.certificate_id
  LEFT JOIN registries r ON r.id = c.registry_id
  LEFT JOIN courses cr ON cr.id = r.course_id
  WHERE a.journal_id = $1
  ORDER BY a.sort_order, a.id;

-- name: AddJournalAttendee :one
  WITH journal_header AS (
      SELECT
          j.id,
          comp.name AS company_name
      FROM training_journals j
      LEFT JOIN companies comp ON comp.id = j.company_id
      WHERE j.id = sqlc.arg(journal_id)
  ),
  student_data AS (
      SELECT
          s.id,
          TRIM(CONCAT_WS(' ', s.lastname, s.firstname, COALESCE(s.secondname, ''))) AS full_name_snapshot,
          s.birthdate AS birthdate_snapshot,
          COALESCE(c.name, jh.company_name) AS company_name_snapshot
      FROM students s
      CROSS JOIN journal_header jh
      LEFT JOIN companies c ON c.id = s.company_id
      WHERE s.id = sqlc.arg(student_id)
  ),
  next_sort_order AS (
      SELECT
          COALESCE(MAX(a.sort_order), 0) + 1 AS value
      FROM training_journal_attendees a
      WHERE a.journal_id = sqlc.arg(journal_id)
  ),
  inserted AS (
      INSERT INTO training_journal_attendees (
          journal_id,
          student_id,
          full_name_snapshot,
          birthdate_snapshot,
          company_name_snapshot,
          sort_order
      )
      SELECT
          sqlc.arg(journal_id),
          sqlc.arg(student_id),
          sd.full_name_snapshot,
          sd.birthdate_snapshot,
          sd.company_name_snapshot,
          nso.value
      FROM student_data sd
      CROSS JOIN next_sort_order nso
      RETURNING
          id,
          journal_id,
          student_id,
          certificate_id,
          full_name_snapshot,
          birthdate_snapshot,
          company_name_snapshot,
          sort_order,
          created_at
  )
  SELECT
      i.id,
      i.journal_id,
      i.student_id,
      i.certificate_id,
      i.full_name_snapshot,
      i.birthdate_snapshot,
      i.company_name_snapshot,
      i.sort_order,
      i.created_at,
      c.date AS certificate_date,
      r.year AS certificate_registry_year,
      COALESCE(r.number::bigint, 0::bigint) AS certificate_registry_number,
      cr.symbol AS certificate_course_symbol
  FROM inserted i
  LEFT JOIN certificates c ON c.id = i.certificate_id
  LEFT JOIN registries r ON r.id = c.registry_id
  LEFT JOIN courses cr ON cr.id = r.course_id;

-- name: UpdateJournalAttendeeCertificate :one
  WITH attendee_source AS (
      SELECT
          a.id,
          a.journal_id,
          a.student_id
      FROM training_journal_attendees a
      WHERE a.journal_id = sqlc.arg(journal_id)
        AND a.id = sqlc.arg(attendee_id)
  ),
  valid_certificate AS (
      SELECT c.id
      FROM attendee_source src
      JOIN training_journals j ON j.id = src.journal_id
      JOIN certificates c ON c.id = sqlc.narg(certificate_id)::bigint
          AND c.student_id = src.student_id
          AND c.deleted_at IS NULL
      JOIN registries r ON r.id = c.registry_id
      WHERE r.course_id = j.course_id
  ),
  updated AS (
      UPDATE training_journal_attendees a
      SET certificate_id = CASE
          WHEN sqlc.narg(certificate_id)::bigint IS NULL THEN NULL
          ELSE (SELECT vc.id FROM valid_certificate vc)
      END
      FROM attendee_source src
      WHERE a.id = src.id
        AND a.journal_id = src.journal_id
        AND (
            sqlc.narg(certificate_id)::bigint IS NULL
            OR EXISTS (SELECT 1 FROM valid_certificate)
        )
      RETURNING
          a.id,
          a.journal_id,
          a.student_id,
          a.certificate_id,
          a.full_name_snapshot,
          a.birthdate_snapshot,
          a.company_name_snapshot,
          a.sort_order,
          a.created_at
  )
  SELECT
      u.id,
      u.journal_id,
      u.student_id,
      u.certificate_id,
      u.full_name_snapshot,
      u.birthdate_snapshot,
      u.company_name_snapshot,
      u.sort_order,
      u.created_at,
      c.date AS certificate_date,
      r.year AS certificate_registry_year,
      COALESCE(r.number::bigint, 0::bigint) AS certificate_registry_number,
      cr.symbol AS certificate_course_symbol
  FROM updated u
  LEFT JOIN certificates c ON c.id = u.certificate_id
  LEFT JOIN registries r ON r.id = c.registry_id
  LEFT JOIN courses cr ON cr.id = r.course_id;

-- name: GetJournalAttendeeForCertificateGeneration :one
  SELECT
      a.id AS attendee_id,
      a.journal_id,
      a.student_id,
      a.certificate_id,
      j.course_id,
      j.date_start,
      j.date_end
  FROM training_journal_attendees a
  JOIN training_journals j ON j.id = a.journal_id
  WHERE a.journal_id = $1
    AND a.id = $2;

-- name: DeleteJournalAttendee :execrows
  DELETE FROM training_journal_attendees
  WHERE journal_id = $1
    AND id = $2;

-- name: ListJournalSessions :many
  SELECT
      s.id,
      s.journal_id,
      s.session_date,
      s.start_time,
      s.end_time,
      s.hours,
      s.topic,
      s.trainer_name,
      s.sort_order,
      s.created_at
  FROM training_journal_sessions s
  WHERE s.journal_id = $1
  ORDER BY s.sort_order, s.id;

-- name: GenerateJournalSessionsFromCourse :execrows
  WITH journal_source AS (
      SELECT
          j.id,
          j.date_start,
          j.organizer_name,
          COALESCE(c.courseprogram::jsonb, '[]'::jsonb) AS course_program
      FROM training_journals j
      JOIN courses c ON c.id = j.course_id
      WHERE j.id = $1
  ),
  program_entries AS (
      SELECT
          js.id AS journal_id,
          js.date_start AS session_date,
          js.organizer_name AS trainer_name,
          TRIM(COALESCE(entry.item->>'Subject', '')) AS topic,
          COALESCE(NULLIF(entry.item->>'TheoryTime', '')::numeric, 0)
              + COALESCE(NULLIF(entry.item->>'PracticeTime', '')::numeric, 0) AS hours,
          entry.ordinality::integer AS sort_order
      FROM journal_source js
      CROSS JOIN LATERAL jsonb_array_elements(js.course_program)
          WITH ORDINALITY AS entry(item, ordinality)
  ),
  filtered_entries AS (
      SELECT
          journal_id,
          session_date,
          trainer_name,
          COALESCE(NULLIF(topic, ''), 'Temat ' || sort_order::text) AS topic,
          hours,
          sort_order
      FROM program_entries
      WHERE topic <> '' OR hours > 0
  )
  INSERT INTO training_journal_sessions (
      journal_id,
      session_date,
      hours,
      topic,
      trainer_name,
      sort_order
  )
  SELECT
      fe.journal_id,
      fe.session_date,
      fe.hours,
      fe.topic,
      fe.trainer_name,
      fe.sort_order
  FROM filtered_entries fe
  WHERE NOT EXISTS (
      SELECT 1
      FROM training_journal_sessions existing
      WHERE existing.journal_id = fe.journal_id
  );

-- name: UpdateJournalSession :one
  UPDATE training_journal_sessions
  SET
      session_date = sqlc.arg(session_date),
      trainer_name = sqlc.arg(trainer_name)
  WHERE journal_id = sqlc.arg(journal_id)
    AND id = sqlc.arg(session_id)
  RETURNING
      id,
      journal_id,
      session_date,
      start_time,
      end_time,
      hours,
      topic,
      trainer_name,
      sort_order,
      created_at;

-- name: ListJournalAttendance :many
  SELECT
      a.id,
      a.journal_session_id,
      a.journal_attendee_id,
      a.present,
      a.created_at,
      a.updated_at
  FROM training_journal_attendance a
  JOIN training_journal_sessions s ON s.id = a.journal_session_id
  WHERE s.journal_id = $1
  ORDER BY a.journal_session_id, a.journal_attendee_id;

-- name: UpsertJournalAttendance :one
  WITH valid_pair AS (
      SELECT
          s.id AS journal_session_id,
          a.id AS journal_attendee_id
      FROM training_journal_sessions s
      JOIN training_journal_attendees a ON a.journal_id = s.journal_id
      WHERE s.journal_id = sqlc.arg(journal_id)
        AND s.id = sqlc.arg(journal_session_id)
        AND a.id = sqlc.arg(journal_attendee_id)
  )
  INSERT INTO training_journal_attendance (
      journal_session_id,
      journal_attendee_id,
      present
  )
  SELECT
      vp.journal_session_id,
      vp.journal_attendee_id,
      sqlc.arg(present)
  FROM valid_pair vp
  ON CONFLICT (journal_session_id, journal_attendee_id)
  DO UPDATE SET
      present = EXCLUDED.present,
      updated_at = now()
  RETURNING
      id,
      journal_session_id,
      journal_attendee_id,
      present,
      created_at,
      updated_at;

-- name: UpdateJournalHeader :one
  WITH updated AS (
      UPDATE training_journals AS tj
      SET
          company_id = sqlc.arg(company_id),
          title = sqlc.arg(title),
          organizer_name = sqlc.arg(organizer_name),
          organizer_address = sqlc.arg(organizer_address),
          location = sqlc.arg(location),
          form_of_training = sqlc.arg(form_of_training),
          legal_basis = sqlc.arg(legal_basis),
          date_start = sqlc.arg(date_start),
          date_end = sqlc.arg(date_end),
          notes = sqlc.arg(notes),
          updated_at = now()
      WHERE tj.id = sqlc.arg(journal_id)
      RETURNING
          tj.id,
          tj.course_id,
          tj.company_id,
          tj.title,
          tj.course_symbol,
          tj.organizer_name,
          tj.organizer_address,
          tj.location,
          tj.form_of_training,
          tj.legal_basis,
          tj.date_start,
          tj.date_end,
          tj.total_hours,
          tj.notes,
          tj.status,
          tj.created_by_user_id,
          tj.created_at,
          tj.updated_at,
          tj.closed_at
  )
  SELECT
      j.id,
      j.course_id,
      c.name AS course_name,
      j.company_id,
      comp.name AS company_name,
      j.title,
      j.course_symbol,
      j.organizer_name,
      j.organizer_address,
      j.location,
      j.form_of_training,
      j.legal_basis,
      j.date_start,
      j.date_end,
      j.total_hours,
      j.notes,
      j.status,
      j.created_by_user_id,
      j.created_at,
      j.updated_at,
      j.closed_at,
      (
          SELECT COUNT(*)::bigint
          FROM training_journal_attendees a
          WHERE a.journal_id = j.id
      ) AS attendees_count,
      (
          SELECT COUNT(*)::bigint
          FROM training_journal_sessions s
          WHERE s.journal_id = j.id
      ) AS sessions_count
  FROM updated j
  JOIN courses c ON c.id = j.course_id
  LEFT JOIN companies comp ON comp.id = j.company_id;


-- name: GetJournalAttendanceScanMeta :one
  SELECT
      id,
      journal_id,
      file_name,
      content_type,
      file_size,
      uploaded_by_user_id,
      created_at,
      updated_at
  FROM training_journal_attendance_scans
  WHERE journal_id = $1;

-- name: GetJournalAttendanceScanFile :one
  SELECT
      id,
      journal_id,
      file_name,
      content_type,
      file_data
  FROM training_journal_attendance_scans
  WHERE journal_id = $1;

-- name: UpsertJournalAttendanceScan :one
  INSERT INTO training_journal_attendance_scans (
      journal_id,
      file_name,
      content_type,
      file_size,
      file_data,
      uploaded_by_user_id
  ) VALUES (
      $1, $2, $3, $4, $5, $6
  )
  ON CONFLICT (journal_id) DO UPDATE
  SET
      file_name = EXCLUDED.file_name,
      content_type = EXCLUDED.content_type,
      file_size = EXCLUDED.file_size,
      file_data = EXCLUDED.file_data,
      uploaded_by_user_id = EXCLUDED.uploaded_by_user_id,
      updated_at = now()
  RETURNING
      id,
      journal_id,
      file_name,
      content_type,
      file_size,
      uploaded_by_user_id,
      created_at,
      updated_at;

-- name: DeleteJournalAttendanceScan :execrows
  DELETE FROM training_journal_attendance_scans
  WHERE journal_id = $1;


