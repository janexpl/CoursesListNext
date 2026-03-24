package journals

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pdfutil"
)

type journalCourseProgramEntry struct {
	Subject      string `json:"Subject"`
	TheoryTime   string `json:"TheoryTime"`
	PracticeTime string `json:"PracticeTime"`
}

type journalProgramSplit struct {
	Theory   string
	Practice string
}

var renderJournalPDF = pdfutil.RenderHTMLToPDF

func buildJournalPDFHTML(
	journal sqlc.GetJournalByIDRow,
	course sqlc.Course,
	attendees []sqlc.ListJournalAttendeesRow,
	sessions []sqlc.TrainingJournalSession,
	attendance []sqlc.TrainingJournalAttendance,
) string {
	attendanceByKey := make(map[string]bool, len(attendance))
	for _, entry := range attendance {
		attendanceByKey[journalAttendanceKey(entry.JournalSessionID, entry.JournalAttendeeID)] = entry.Present
	}

	programSplitBySortOrder := buildJournalProgramSplitBySortOrder(course.Courseprogram, sessions)
	participantRowsHTML := buildJournalParticipantRowsHTML(attendees)
	programRowsHTML := buildJournalProgramRowsHTML(sessions, programSplitBySortOrder)
	attendanceHeadHTML := buildJournalAttendanceHeadHTML(sessions)
	attendanceRowsHTML := buildJournalAttendanceRowsHTML(attendees, sessions, attendanceByKey)

	return `<!doctype html>
<html lang="pl">
  <head>
    <meta charset="utf-8">
    <title>` + html.EscapeString(journal.Title) + `</title>
    <style>
      @page {
        size: A4 portrait;
        margin: 14mm;
      }

      @page attendance-landscape {
        size: A4 landscape;
        margin: 12mm;
      }

      * {
        box-sizing: border-box;
      }

      html,
      body {
        margin: 0;
        padding: 0;
        background: white;
        color: #0f172a;
        font-family: "Liberation Sans", "Times New Roman", "Liberation Serif", Times, serif;
      }

      body {
        font-size: 12px;
        line-height: 1.35;
      }

      .document {
        padding: 12px;
      }

      .print-sheet {
        page-break-after: always;
        break-after: page;
      }

      .print-sheet:last-of-type {
        page-break-after: auto;
        break-after: auto;
      }

      .sheet-header {
        margin-bottom: 18px;
        padding-bottom: 14px;
        border-bottom: 1px solid #cbd5e1;
      }

      .status-badge {
        display: inline-flex;
        margin-bottom: 10px;
        border: 1px solid #cbd5e1;
        border-radius: 999px;
        padding: 4px 10px;
        font-size: 13px;
        font-weight: 600;
        letter-spacing: 0.08em;
        color: #475569;
      }

      .course-symbol {
        margin-left: 8px;
        font-size: 13px;
        letter-spacing: 0.14em;
        color: #64748b;
        text-transform: uppercase;
      }

      h1,
      h2 {
        margin: 0;
        color: #0f172a;
      }

      h1 {
        font-size: 35px;
        line-height: 1.2;
      }

      h2 {
        font-size: 24px;
        line-height: 1.25;
      }

      .subtitle {
        margin-top: 6px;
        font-size: 15px;
        color: #475569;
      }

      .section-lead {
        margin: 4px 0 14px;
        font-size: 12px;
        color: #64748b;
      }

      .details-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 12px 18px;
        font-size: 18px;
      }

      .details-grid dt {
        margin: 15px 0 0;
        font-size: 12px;
        letter-spacing: 0.12em;
        color: #64748b;
        text-transform: uppercase;
      }

      .details-grid dd {
        margin: 0;
        color: #0f172a;
      }

      .full-width {
        grid-column: 1 / -1;
      }

      .print-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
      }

      .print-table th,
      .print-table td {
        border: 1px solid #cbd5e1;
        padding: 8px 10px;
        vertical-align: top;
      }

      .print-table thead th {
        background: #f8fafc;
        font-size: 10px;
        font-weight: 600;
        letter-spacing: 0.03em;
        color: #475569;
        text-transform: capitalize;
      }

      .program-table {
        table-layout: fixed;
      }

      .program-table col:nth-child(1) {
        width: 7%;
      }

      .program-table col:nth-child(2) {
        width: 14%;
      }

      .program-table col:nth-child(3) {
        width: 31%;
      }

      .program-table col:nth-child(4),
      .program-table col:nth-child(5) {
        width: 14%;
      }

      .program-table col:nth-child(6) {
        width: 20%;
      }

      .program-table td,
      .program-table th {
        word-break: break-word;
      }

      .trainer-signature {
        padding-top: 96px;
      }

      .trainer-signature__line {
        width: 220px;
        border-top: 1px solid #64748b;
        padding-top: 6px;
        font-size: 10px;
        text-align: center;
        color: #475569;
      }

      .attendance-sheet {
        page: attendance-landscape;
        page-break-before: always;
        break-before: page;
      }

      .attendance-table th:not(:first-child),
      .attendance-table td:not(:first-child) {
        min-width: 64px;
        width: 64px;
        text-align: center;
      }

      .attendance-table th:first-child,
      .attendance-table td:first-child {
        min-width: 165px;
        width: 165px;
      }

      .attendance-heading {
        font-size: 8px;
        line-height: 1.15;
      }

      .attendee-cell {
        font-weight: 500;
      }
    </style>
  </head>
  <body>
    <div class="document">
      <article class="print-sheet">
        <div class="sheet-header">
          <div class="status-badge">` + html.EscapeString(journalStatusLabel(journal.Status)) + `</div>
          <span class="course-symbol">` + html.EscapeString(journal.CourseSymbol) + `</span>
          <h1>` + html.EscapeString(journal.Title) + `</h1>
        </div>

        <dl class="details-grid">
          <div>
            <dt>Organizator</dt>
            <dd>` + html.EscapeString(journal.OrganizerName) + `</dd>
          </div>
          <div>
            <dt>Miejsce</dt>
            <dd>` + html.EscapeString(journal.Location) + `</dd>
          </div>
          <div>
            <dt>Forma szkolenia</dt>
            <dd>` + html.EscapeString(journal.FormOfTraining) + `</dd>
          </div>
          <div>
            <dt>Firma</dt>
            <dd>` + html.EscapeString(textOrFallback(journal.CompanyName, "Bez przypisanej firmy")) + `</dd>
          </div>
          <div>
            <dt>Termin</dt>
            <dd>` + html.EscapeString(formatJournalPrintDate(journal.DateStart)) + ` - ` + html.EscapeString(formatJournalPrintDate(journal.DateEnd)) + `</dd>
          </div>
          <div>
            <dt>Liczba godzin</dt>
            <dd>` + html.EscapeString(buildHoursLabel(formatNumeric(journal.TotalHours))) + `</dd>
          </div>
          <div class="full-width">
            <dt>Podstawa prawna</dt>
            <dd>` + html.EscapeString(journal.LegalBasis) + `</dd>
          </div>
          <div class="full-width">
            <dt>Adres organizatora</dt>
            <dd>` + html.EscapeString(textOrFallback(journal.OrganizerAddress, "Brak adresu organizatora")) + `</dd>
          </div>
          <div class="full-width">
            <dt>Notatki</dt>
            <dd>` + html.EscapeString(textOrFallback(journal.Notes, "Brak notatek")) + `</dd>
          </div>
        </dl>
      </article>

      <article class="print-sheet">
        <h2>Lista uczestników</h2>
        <p class="section-lead">Snapshot uczestników przypisanych do tego szkolenia.</p>
        <table class="print-table">
          <thead>
            <tr>
              <th>Lp.</th>
              <th>Uczestnik</th>
              <th>Data urodzenia</th>
              <th>Firma</th>
              <th>Zaświadczenie</th>
            </tr>
          </thead>
          <tbody>
            ` + participantRowsHTML + `
          </tbody>
        </table>
      </article>

      <article class="print-sheet">
        <h2>Program szkolenia</h2>
        <p class="section-lead">Tematy, prowadzący i godziny przypisane do dziennika.</p>
        <table class="print-table program-table">
          <colgroup>
            <col>
            <col>
            <col>
            <col>
            <col>
            <col>
          </colgroup>
          <thead>
            <tr>
              <th>Lp.</th>
              <th>Data</th>
              <th>Temat</th>
              <th>Godziny teorii</th>
              <th>Godziny praktyki</th>
              <th>Prowadzący</th>
            </tr>
          </thead>
          <tbody>
            ` + programRowsHTML + `
          </tbody>
        </table>

        <div class="trainer-signature">
          <div class="trainer-signature__line">Podpis wykładowcy</div>
        </div>
      </article>

      <article class="print-sheet attendance-sheet">
        <h2>Lista obecności</h2>
        <p class="section-lead">Obecność uczestników dla poszczególnych pozycji programu.</p>
        <table class="print-table attendance-table">
          <thead>
            <tr>
              <th>Uczestnik</th>
              ` + attendanceHeadHTML + `
            </tr>
          </thead>
          <tbody>
            ` + attendanceRowsHTML + `
          </tbody>
        </table>
      </article>
    </div>
  </body>
</html>`
}

func buildJournalParticipantRowsHTML(attendees []sqlc.ListJournalAttendeesRow) string {
	if len(attendees) == 0 {
		return `<tr><td colspan="5">Brak uczestników przypisanych do dziennika.</td></tr>`
	}

	rows := make([]string, 0, len(attendees))
	for index, attendee := range attendees {
		certificateLabel := "Brak"
		if attendee.CertificateID.Valid {
			certificateLabel = buildJournalCertificateNumber(attendee)
		}

		rows = append(rows, fmt.Sprintf(
			"<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			index+1,
			html.EscapeString(attendee.FullNameSnapshot),
			html.EscapeString(formatJournalPrintDate(attendee.BirthdateSnapshot)),
			html.EscapeString(textOrFallback(attendee.CompanyNameSnapshot, "Brak firmy")),
			html.EscapeString(certificateLabel),
		))
	}

	return strings.Join(rows, "")
}

func buildJournalProgramRowsHTML(
	sessions []sqlc.TrainingJournalSession,
	programSplitBySortOrder map[int32]journalProgramSplit,
) string {
	if len(sessions) == 0 {
		return `<tr><td colspan="6">Brak pozycji programu w dzienniku.</td></tr>`
	}

	rows := make([]string, 0, len(sessions))
	for index, session := range sessions {
		split := programSplitBySortOrder[session.SortOrder]
		if split.Theory == "" && split.Practice == "" {
			split = journalProgramSplit{
				Theory:   formatNumeric(session.Hours),
				Practice: "0",
			}
		}

		rows = append(rows, fmt.Sprintf(
			"<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			index+1,
			html.EscapeString(formatJournalPrintDate(session.SessionDate)),
			html.EscapeString(session.Topic),
			html.EscapeString(split.Theory),
			html.EscapeString(split.Practice),
			html.EscapeString(session.TrainerName),
		))
	}

	return strings.Join(rows, "")
}

func buildJournalAttendanceHeadHTML(sessions []sqlc.TrainingJournalSession) string {
	if len(sessions) == 0 {
		return `<th>Brak sesji</th>`
	}

	headers := make([]string, 0, len(sessions))
	for _, session := range sessions {
		headers = append(headers, fmt.Sprintf(
			"<th><div class='attendance-heading'><strong>%d.</strong><br>%s<br>%s</div></th>",
			session.SortOrder,
			html.EscapeString(formatJournalPrintDate(session.SessionDate)),
			html.EscapeString(shortenJournalAttendanceTopic(session.Topic, 28)),
		))
	}

	return strings.Join(headers, "")
}

func buildJournalAttendanceRowsHTML(
	attendees []sqlc.ListJournalAttendeesRow,
	sessions []sqlc.TrainingJournalSession,
	attendanceByKey map[string]bool,
) string {
	if len(attendees) == 0 {
		return `<tr><td colspan="2">Brak uczestników przypisanych do dziennika.</td></tr>`
	}
	if len(sessions) == 0 {
		return `<tr><td colspan="2">Brak sesji przypisanych do dziennika.</td></tr>`
	}

	rows := make([]string, 0, len(attendees))
	for _, attendee := range attendees {
		cells := make([]string, 0, len(sessions))
		for _, session := range sessions {
			mark := ""
			if attendanceByKey[journalAttendanceKey(session.ID, attendee.ID)] {
				mark = "X"
			}
			cells = append(cells, "<td>"+mark+"</td>")
		}

		rows = append(rows, fmt.Sprintf(
			"<tr><td class='attendee-cell'>%s</td>%s</tr>",
			html.EscapeString(attendee.FullNameSnapshot),
			strings.Join(cells, ""),
		))
	}

	return strings.Join(rows, "")
}

func buildJournalProgramSplitBySortOrder(courseProgram []byte, sessions []sqlc.TrainingJournalSession) map[int32]journalProgramSplit {
	result := make(map[int32]journalProgramSplit, len(sessions))
	if len(courseProgram) == 0 {
		return result
	}

	var entries []journalCourseProgramEntry
	if err := json.Unmarshal(courseProgram, &entries); err != nil {
		return result
	}

	for _, session := range sessions {
		index := int(session.SortOrder) - 1
		if index < 0 || index >= len(entries) {
			continue
		}
		entry := entries[index]
		result[session.SortOrder] = journalProgramSplit{
			Theory:   formatJournalProgramHours(entry.TheoryTime),
			Practice: formatJournalProgramHours(entry.PracticeTime),
		}
	}

	return result
}

func formatJournalProgramHours(value string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if normalized == "" {
		return "0"
	}

	var parsed pgtype.Numeric
	if err := parsed.Scan(normalized); err != nil {
		return "0"
	}

	formatted := formatNumeric(parsed)
	if formatted == "" {
		return "0"
	}

	return strings.Trim(formatted, `"`)
}

func formatJournalPrintDate(value pgtype.Date) string {
	if !value.Valid {
		return "Brak"
	}

	return value.Time.Format("02.01.2006")
}

func journalStatusLabel(value string) string {
	if value == "closed" {
		return "Zamknięty"
	}

	return "Roboczy"
}

func buildHoursLabel(value string) string {
	normalized := strings.Trim(value, `"`)
	if normalized == "" {
		return "0 h"
	}

	return normalized + " h"
}

func textOrFallback(value pgtype.Text, fallback string) string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return fallback
	}

	return value.String
}

func buildJournalCertificateNumber(attendee sqlc.ListJournalAttendeesRow) string {
	registryNumber := strings.Trim(fmt.Sprint(attendee.CertificateRegistryNumber), `"`)
	if attendee.CertificateRegistryYear.Int64 == 0 || attendee.CertificateCourseSymbol.String == "" || registryNumber == "" || registryNumber == "<nil>" {
		return "Brak"
	}

	return fmt.Sprintf("%s/%s/%d", registryNumber, attendee.CertificateCourseSymbol.String, attendee.CertificateRegistryYear.Int64)
}

func journalAttendanceKey(sessionID, attendeeID int64) string {
	return fmt.Sprintf("%d:%d", sessionID, attendeeID)
}

func shortenJournalAttendanceTopic(topic string, maxLength int) string {
	normalized := strings.TrimSpace(topic)
	if len(normalized) <= maxLength {
		return normalized
	}

	shortened := strings.TrimSpace(normalized[:maxLength])
	lastSpace := strings.LastIndex(shortened, " ")
	if lastSpace > maxLength/2 {
		shortened = shortened[:lastSpace]
	}

	return shortened + "..."
}

func buildJournalPDFFilename(journal sqlc.GetJournalByIDRow) string {
	safeTitle := strings.NewReplacer("/", "-", "\\", "-", " ", "_").Replace(strings.TrimSpace(journal.Title))
	if safeTitle == "" {
		safeTitle = fmt.Sprintf("dziennik-%d", journal.ID)
	}

	return safeTitle + ".pdf"
}
