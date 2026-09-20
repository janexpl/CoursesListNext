package certificates

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pdfutil"
	"github.com/janexpl/CoursesListNext/api/internal/qrcode"
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

// buildCertificatePDFHTML składa wydruk. verificationURLTemplate to wzorzec adresu
// publicznej weryfikacji; pusty oznacza wydruk bez kodu QR.
func buildCertificatePDFHTML(certificate sqlc.GetCertificateByIDRow, verificationURLTemplate string) string {
	qrHTML := buildVerificationQR(certificate, verificationURLTemplate)
	template, qrPlaced := substituteCertificateTemplate(certificate, qrHTML)
	front := buildDuplicateAnnotation(certificate) + template
	// Szablon bez znacznika (tak wygląda większość istniejących kursów) dostaje kod
	// w prawym dolnym rogu pierwszej strony.
	if qrHTML != "" && !qrPlaced {
		front = `<div class="cert-front">` + front + `<div class="qr-corner">` + qrHTML + `</div></div>`
	}
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
      margin: 0;
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

    .duplicate {
      text-align: right;
      margin-bottom: 8mm;
    }

    .duplicate-label {
      display: block;
      font-size: 20px;
      font-weight: 700;
      letter-spacing: 0.22em;
    }

    .duplicate-date {
      display: block;
      font-size: 12px;
    }

    /* Element inline, nie blokowy: szablony wstawiają znacznik wewnątrz akapitu,
       a <div> w <p> parser HTML wyrzuca poza akapit - kod przestałby wtedy słuchać
       wyrównania ustawionego przez autora szablonu. */
    .qr-code {
      display: inline-block;
    }

    .qr-code img {
      width: 24mm;
      height: 24mm;
      display: block;
    }

    /* Kod w rogu pierwszej strony. Pozycjonowanie bezwzględne w kontenerze o zadanej
       wysokości, a nie position: fixed - chromium drukuje "fixed" tylko na pierwszej
       stronie, a wkhtmltopdf powtarza je na każdej. */
    /* padding-bottom rezerwuje pasek na kod: treść płynie w obszarze zawartości,
       a kod siedzi w pasku paddingu, więc nie da się go już przykryć podpisem.
       min-height plus padding dają razem wysokość strony. */
    .cert-front {
      position: relative;
      min-height: 202mm;
      padding-bottom: 28mm;
    }

    .qr-corner {
      position: absolute;
      right: 0;
      bottom: 0;
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

// buildDuplicateAnnotation drukuje adnotację wtórnika. Duplikat to ten sam dokument
// o tym samym numerze rejestru, więc odróżnia go od oryginału wyłącznie ta adnotacja:
// słowo DUPLIKAT i data jego wystawienia. Napis jest po polsku także na wydrukach
// w innych językach - to adnotacja na polskim dokumencie.
func buildDuplicateAnnotation(certificate sqlc.GetCertificateByIDRow) string {
	if !certificate.DuplicateIssuedAt.Valid {
		return ""
	}

	return `<div class="duplicate">` +
		`<span class="duplicate-label">DUPLIKAT</span>` +
		`<span class="duplicate-date">Data wystawienia duplikatu: ` +
		html.EscapeString(certificate.DuplicateIssuedAt.Time.Format("02.01.2006")) +
		`</span></div>`
}

// buildVerificationQR zwraca blok z kodem QR prowadzącym do publicznej weryfikacji
// dokumentu albo pusty string, gdy adres nie jest skonfigurowany. Błąd generowania
// kodu nie przerywa wydruku - dokument bez QR jest lepszy niż brak dokumentu.
func buildVerificationQR(certificate sqlc.GetCertificateByIDRow, verificationURLTemplate string) string {
	url := qrcode.VerificationURL(verificationURLTemplate, certificate.VerificationCode)
	if url == "" {
		return ""
	}

	dataURI, err := qrcode.PNGDataURI(url)
	if err != nil {
		log.Printf("failed to build verification QR code for certificate %d: %v", certificate.ID, err)
		return ""
	}

	return `<span class="qr-code"><img src="` + dataURI + `" alt="Kod QR do weryfikacji zaświadczenia"></span>`
}

// substituteCertificateTemplate podmienia znaczniki w szablonie kursu. Zwraca też
// informację, czy szablon zawierał znacznik kodu QR - jeśli nie, wywołujący dokłada
// kod w rogu strony.
func substituteCertificateTemplate(certificate sqlc.GetCertificateByIDRow, qrHTML string) (string, bool) {
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

	// rawValues omijają html.EscapeString, więc wolno tu wkładać WYŁĄCZNIE HTML zbudowany
	// w tym pakiecie z danych, których nie kontroluje użytkownik. Kod QR to obrazek
	// data: URI wygenerowany z kodu weryfikacyjnego - nic z bazy tu nie trafia.
	rawValues := map[string]string{}
	if qrHTML != "" {
		rawValues["kod_qr"] = qrHTML
	}

	qrPlaced := false
	substituted := certificatePlaceholderPattern.ReplaceAllStringFunc(certificate.CertFrontPage, func(token string) string {
		matches := certificatePlaceholderPattern.FindStringSubmatch(token)
		if len(matches) != 2 {
			return ""
		}

		normalized := strings.Join(strings.Fields(matches[1]), "")
		if raw, ok := rawValues[normalized]; ok {
			qrPlaced = true
			return raw
		}
		return html.EscapeString(values[normalized])
	})

	return substituted, qrPlaced
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

		fmt.Fprintf(&rows, "<tr><td>%d</td><td>%s</td><td class='hour'>%s</td><td class='hour'>%s</td></tr>",
			index+1,
			html.EscapeString(entry.Subject),
			html.EscapeString(entry.TheoryTime),
			html.EscapeString(entry.PracticeTime))
	}

	fmt.Fprintf(&rows, "<tr><td colspan='2'>%s</td><td class='hour'>%.1f</td><td class='hour'>%.1f</td></tr>",
		html.EscapeString(labels.Total),
		theorySum,
		practiceSum)

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
