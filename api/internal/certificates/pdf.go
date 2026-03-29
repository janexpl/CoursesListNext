package certificates

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pdfutil"
)

type courseProgramEntry struct {
	Subject      string `json:"Subject"`
	TheoryTime   string `json:"TheoryTime"`
	PracticeTime string `json:"PracticeTime"`
}

type courseProgramPageLabels struct {
	Index            string
	Subject          string
	TheoryHours      string
	PracticeHours    string
	Total            string
	HTMLLanguageCode string
}

var certificatePlaceholderPattern = regexp.MustCompile(`{{(.*?)}}`)

var renderCertificatePDF = pdfutil.RenderHTMLToPDF

func buildCertificatePDFHTML(certificate sqlc.GetCertificateByIDRow) string {
	front := substituteCertificateTemplate(certificate)
	back := buildCourseProgramPage(certificate.CourseProgram, certificate.LanguageCode)
	labels := getCourseProgramPageLabels(certificate.LanguageCode)

	return `<!doctype html>
<html lang="` + labels.HTMLLanguageCode + `">
<head>
  <meta charset="utf-8">
  <title>ZAŚWIADCZENIE</title>
  <style>
    @page {
      size: A4 portrait;
      margin: 0;
    }

    html, body {
      margin: -;
      padding: 0;
      color: #0f172a;
      background: white;
      font-family: "Liberation Serif", "Times New Roman", Times, serif !important;
    }

    body {
      display: block;
      margin: 15mm;
      padding: 14mm 16mm;
      line-height: 1.4;
    }

    body, body *,
    p, div, span, table, th, td, h1, h2, h3, h4, h5, h6, li, strong, em {
      font-family: "Times New Roman", "Liberation Serif", Times, serif !important;
    }

    .break {
      page-break-before: always;
      display: block;
    }

    .spacer {
      height: 25mm;
    }

    h1, h2, h3, h4, h5, h6 {
      margin: 0 0 0.45rem;
      line-height: 1.2;
      color: #020617;
    }

    h1 {
      font-size: 54px;
      font-weight: 700;
      letter-spacing: 0.02em;
    }

    h2 {
      font-size: 24px;
      font-weight: 700;
    }

    h3 {
      font-size: 18px;
      font-weight: 700;
    }

    p {
      margin: 0 0 0.32rem;
      font-size: 14px;
      line-height: 1.25;
    }

    ul, ol {
      margin: 0 0 0.45rem;
      padding-left: 1.25rem;
    }

    img {
      max-width: 100%;
      height: auto;
    }

    table {
      width: 100%;
      border-collapse: collapse;
    }

    .hour {
      text-align: center;
      white-space: nowrap;
    }

    .program-table {
      table-layout: fixed;
    }

    .col-lp {
      width: 7%;
    }

    .col-subject {
      width: 61%;
    }

    .col-hours {
      width: 16%;
    }

    table, th, td {
      padding: 5px;
      font-size: 11px;
      line-height: 1.35;
      border: 1px solid black;
    }

    th {
      background: #f8fafc;
    }
  </style>
</head>
<body>` + front + back + `</body>
</html>`
}

func substituteCertificateTemplate(certificate sqlc.GetCertificateByIDRow) string {
	values := map[string]string{
		"imie":                certificate.StudentFirstname,
		"drugie_imie":         certificate.StudentSecondname.String,
		"nazwisko":            certificate.StudentLastname,
		"pesel":               certificate.StudentPesel.String,
		"data_urodzenia":      formatPolishDate(certificate.StudentBirthdate),
		"miejsce_urodzenia":   certificate.StudentBirthplace,
		"nazwa_kursu":         certificate.CourseName,
		"data_rozpoczecia":    formatPolishDate(certificate.CourseDateStart),
		"data_zakonczenia":    formatPolishDate(certificate.CourseDateEnd),
		"data_wystawienia":    formatPolishDate(certificate.Date),
		"numer_zaswiadczenia": buildCertificateNumber(certificate.RegistryNumber, certificate.CourseSymbol, certificate.RegistryYear),
	}

	return certificatePlaceholderPattern.ReplaceAllStringFunc(certificate.CertFrontPage, func(token string) string {
		matches := certificatePlaceholderPattern.FindStringSubmatch(token)
		if len(matches) != 2 {
			return ""
		}

		normalized := strings.Join(strings.Fields(matches[1]), "")
		return values[normalized]
	})
}

func buildCourseProgramPage(raw string, languageCode string) string {
	if raw == "" {
		return ""
	}

	labels := getCourseProgramPageLabels(languageCode)

	var entries []courseProgramEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil || len(entries) == 0 {
		return ""
	}

	var (
		theorySum   float64
		practiceSum float64
		rows        strings.Builder
	)

	for index, entry := range entries {
		theoryValue := parseFloat(entry.TheoryTime)
		practiceValue := parseFloat(entry.PracticeTime)
		theorySum += theoryValue
		practiceSum += practiceValue

		rows.WriteString(fmt.Sprintf(
			"<tr><td>%d</td><td>%s</td><td class='hour'>%s</td><td class='hour'>%s</td></tr>",
			index+1,
			html.EscapeString(entry.Subject),
			html.EscapeString(entry.TheoryTime),
			html.EscapeString(entry.PracticeTime),
		))
	}

	rows.WriteString(fmt.Sprintf(
		"<tr><td colspan='2'>%s</td><td class='hour'>%.1f</td><td class='hour'>%.1f</td></tr>",
		html.EscapeString(labels.Total),
		theorySum,
		practiceSum,
	))

	return `
<div class="break"></div>
<div class="spacer"></div>
<table class="program-table">
  <colgroup>
    <col class="col-lp">
    <col class="col-subject">
    <col class="col-hours">
    <col class="col-hours">
  </colgroup>
  <thead>
    <tr>
	      <th>` + html.EscapeString(labels.Index) + `</th>
	      <th>` + html.EscapeString(labels.Subject) + `</th>
	      <th>` + html.EscapeString(labels.TheoryHours) + `</th>
	      <th>` + html.EscapeString(labels.PracticeHours) + `</th>
    </tr>
  </thead>
  <tbody>` + rows.String() + `</tbody>
</table>`
}

func getCourseProgramPageLabels(languageCode string) courseProgramPageLabels {
	switch strings.ToLower(strings.TrimSpace(languageCode)) {
	case "en":
		return courseProgramPageLabels{
			Index:            "No.",
			Subject:          "Training topic",
			TheoryHours:      "Theory hours",
			PracticeHours:    "Practical hours",
			Total:            "TOTAL",
			HTMLLanguageCode: "en",
		}
	case "de":
		return courseProgramPageLabels{
			Index:            "Nr.",
			Subject:          "Schulungsthema",
			TheoryHours:      "Theoriestunden",
			PracticeHours:    "Praxisstunden",
			Total:            "SUMME",
			HTMLLanguageCode: "de",
		}
	case "uk":
		return courseProgramPageLabels{
			Index:            "№",
			Subject:          "Тема навчання",
			TheoryHours:      "Теоретичні години",
			PracticeHours:    "Практичні години",
			Total:            "РАЗОМ",
			HTMLLanguageCode: "uk",
		}
	case "cs":
		return courseProgramPageLabels{
			Index:            "Č.",
			Subject:          "Téma školení",
			TheoryHours:      "Teoretické hodiny",
			PracticeHours:    "Praktické hodiny",
			Total:            "CELKEM",
			HTMLLanguageCode: "cs",
		}
	case "sk":
		return courseProgramPageLabels{
			Index:            "Č.",
			Subject:          "Téma školenia",
			TheoryHours:      "Teoretické hodiny",
			PracticeHours:    "Praktické hodiny",
			Total:            "SPOLU",
			HTMLLanguageCode: "sk",
		}
	case "lt":
		return courseProgramPageLabels{
			Index:            "Nr.",
			Subject:          "Mokymo tema",
			TheoryHours:      "Teorijos valandos",
			PracticeHours:    "Praktikos valandos",
			Total:            "IŠ VISO",
			HTMLLanguageCode: "lt",
		}
	default:
		return courseProgramPageLabels{
			Index:            "Lp.",
			Subject:          "Temat szkolenia",
			TheoryHours:      "Liczba godzin zajęć teoretycznych (wykładów)",
			PracticeHours:    "Liczba godzin zajęć praktycznych (ćwiczeń)",
			Total:            "RAZEM",
			HTMLLanguageCode: "pl",
		}
	}
}

func formatPolishDate(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format("02.01.2006")
}

func buildCertificateNumber(registryNumber int64, courseSymbol string, registryYear int64) string {
	return fmt.Sprintf("%d/%s/%d", registryNumber, courseSymbol, registryYear)
}

func buildCertificateFilename(certificate sqlc.GetCertificateByIDRow) string {
	number := buildCertificateNumber(certificate.RegistryNumber, certificate.CourseSymbol, certificate.RegistryYear)
	safe := strings.NewReplacer("/", "-", " ", "_").Replace(number)
	return "zaswiadczenie-" + safe + ".pdf"
}

func parseFloat(value string) float64 {
	var parsed float64
	fmt.Sscanf(value, "%f", &parsed)
	return parsed
}
