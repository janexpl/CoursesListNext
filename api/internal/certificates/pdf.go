package certificates

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/certassets"
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

// decorImage to jeden nadruk gotowy do wstawienia: obrazek jako data URI i szerokość,
// jaką ma mieć na papierze.
type decorImage struct {
	DataURI string
	WidthMM int
}

// certificateDecor to nadruki dokładane wyłącznie do zaświadczeń platformowych:
// pieczątki, podpis i giloszowe tło. Wartość zerowa znaczy "wydruk dokładnie taki,
// jak przed wprowadzeniem nadruków" - i tak wygląda dla wszystkich dokumentów
// wystawianych z aplikacji webowej i z dziennika.
type certificateDecor struct {
	StampRound    decorImage
	StampCompany  decorImage
	StampPersonal decorImage
	Signature     decorImage
	// Osobne wzory na przód i odwrót: przód ma monogram, odwrót samą ramkę z siatką.
	GuillocheFront string
	GuillocheBack  string
}

// buildCertificatePDFHTML składa wydruk. verificationURLTemplate to wzorzec adresu
// publicznej weryfikacji; pusty oznacza wydruk bez kodu QR. decor niesie nadruki
// zaświadczenia platformowego; wartość zerowa daje wydruk bez nich.
func buildCertificatePDFHTML(certificate sqlc.GetCertificateByIDRow, verificationURLTemplate string, decor certificateDecor) string {
	qrHTML := buildVerificationQR(certificate, verificationURLTemplate)
	template, placed := substituteCertificateTemplate(certificate, buildCertificateRawValues(qrHTML, decor))
	front := buildDuplicateAnnotation(certificate) + template

	// Szablon bez znacznika (tak wygląda większość istniejących kursów) dostaje kod
	// w prawym dolnym rogu pierwszej strony, a nadruki w pasku nad nim.
	corner := ""
	if qrHTML != "" && !placed["kod_qr"] {
		corner = `<div class="qr-corner">` + qrHTML + `</div>`
	}
	marks := buildFallbackMarks(decor, placed)

	if decor.GuillocheFront != "" {
		// Deseń jest elementem treści, nie tłem CSS - tła bywają pomijane przy druku.
		// Treść dostaje własny pozycjonowany kontener, bo element position:absolute
		// maluje się nad elementami niepozycjonowanymi i przykryłby tekst.
		front = guillocheHTML(decor.GuillocheFront, "") + `<div class="cert-body">` + front + `</div>`
	}
	if corner != "" || marks != "" || decor.GuillocheFront != "" {
		classes := "cert-front"
		if marks != "" {
			// Pasek z nadrukami jest wyższy niż sam kod QR, więc treść dostaje mniej miejsca.
			classes += " cert-front--marks"
		}
		front = `<div class="` + classes + `">` + front + marks + corner + `</div>`
	}
	back := wrapCertificateBack(buildCourseProgramPage(certificate.CourseProgram, certificate.LanguageCode), decor.GuillocheBack)

	// Wydruk z tłem układa się w arkuszach o wymiarach strony, a nie w marginesach body.
	// Powód jest w wkhtmltopdf: tło wychodzące poza obszar układu włączało tam globalne
	// zmniejszanie treści (smart shrinking), którego w debianowym buildzie na niezałatanym
	// Qt nie da się wyłączyć przełącznikiem. Wydruki bez tła zostają przy starym układzie.
	bodyClass := ""
	if decor.GuillocheFront != "" {
		bodyClass = "decor"
	}
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

    .qr-caption {
      display: block;
      width: 24mm;
      margin-top: 1mm;
      font-size: 8pt;
      line-height: 1.1;
      text-align: center;
      font-family: Arial;
    }

    /* Kod w rogu pierwszej strony. Pozycjonowanie bezwzględne w kontenerze o zadanej
       wysokości, a nie position: fixed - chromium drukuje "fixed" tylko na pierwszej
       stronie, a wkhtmltopdf powtarza je na każdej. */
    /* padding-bottom rezerwuje pasek na kod: treść płynie w obszarze zawartości,
       a kod siedzi w pasku paddingu, więc nie da się go już przykryć podpisem.
       min-height plus padding dają razem wysokość strony. */
    .cert-front {
      position: relative;
      min-height: 198mm;
      /* 24 mm kodu, podpis pod nim i odstęp od treści. */
      padding-bottom: 32mm;
    }

    .qr-corner {
      position: absolute;
      right: 0;
      bottom: 0;
    }

    /* Pieczątki i podpis. Element inline z tego samego powodu co .qr-code, a szerokość
       przychodzi w atrybucie style, bo ustala ją administrator przy wgrywaniu pliku. */
    .cert-stamp,
    .cert-signature {
      display: inline-block;
      vertical-align: bottom;
    }

    .cert-stamp img,
    .cert-signature img {
      width: 100%;
      height: auto;
      display: block;
    }

    /* Pasek awaryjny dla szablonów bez znaczników. Kończy się 32 mm przed prawą
       krawędzią, bo tam siedzi kod QR (24 mm plus odstęp). */
    .cert-marks {
      position: absolute;
      left: 0;
      right: 32mm;
      bottom: 0;
      white-space: nowrap;
    }

    /* Pasek mieści cztery nadruki obok siebie, a szerokości ustala administrator,
       więc ograniczamy je procentem szerokości paska. Bez tego komplet pieczątek
       w pełnych rozmiarach wyszedłby poza krawędź papieru. */
    .cert-marks .cert-stamp,
    .cert-marks .cert-signature {
      margin-right: 3%;
      max-width: 22%;
    }

    /* Pasek z nadrukami jest wyższy niż sam kod QR, więc treść dostaje mniej miejsca.
       Modyfikator, a nie zmiana .cert-front - wydruki bez nadruków mają zostać takie
       jak dotąd. */
    .cert-front--marks {
      min-height: 190mm;
      padding-bottom: 40mm;
    }

    /* Układ arkuszowy, włączany klasą na body tylko dla wydruków z tłem.

       Zamiast marginesów body każda strona jest kontenerem o wymiarach arkusza, a wcięcia
       daje jego padding. Dzięki temu tło mieści się w układzie zamiast z niego wystawać:
       w wkhtmltopdf element szerszy niż widok włącza zmniejszanie całej treści, którego
       w tamtym buildzie nie wyłącza żaden przełącznik. Wcięcia są te same co wcześniej
       (15 mm marginesu plus 14/16 mm paddingu), więc treść stoi tam, gdzie stała. */
    body.decor {
      margin: 0;
      padding: 0;
      /* Łańcuch wysokości jest tu potrzebny, żeby min-height: 100% na arkuszu miało
         od czego się liczyć - inaczej wkhtmltopdf zostaje przy swojej skali milimetrów
         i tło kończy się na trzech czwartych strony. */
      height: 100%;
    }

    html {
      height: 100%;
    }

    .decor .cert-front,
    .decor .cert-back {
      box-sizing: border-box;
      position: relative;
      width: 100%;
      /* height dla chromium, min-height dla wkhtmltopdf: ten drugi przelicza milimetry
         we własnej skali, ale rozumie wysokość strony wyrażoną procentem. */
      height: 297mm;
      min-height: 100%;
      padding: 29mm 31mm;
    }

    /* Gilosz jako element treści, nie background-image: tła bywają pomijane przy druku
       i przez sterowniki drukarek. Wypełnia arkusz co do krawędzi. */
    .cert-guilloche {
      position: absolute;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      /* Bez tego ogólna reguła img { max-width: 100% } ścisnęłaby wzór do szerokości
         obszaru treści. */
      max-width: none;
    }

    /* Na arkuszu kod QR i pasek nadruków odmierzają się od krawędzi papieru, więc muszą
       wejść do środka o tyle, ile wynosi wcięcie treści. */
    .decor .qr-corner {
      right: 31mm;
      bottom: 29mm;
    }

    .decor .cert-marks {
      left: 31mm;
      right: 63mm;
      bottom: 29mm;
    }

    .cert-back {
      page-break-before: always;
      break-before: page;
    }

    /* Treść leży nad deseniem. Element pozycjonowany maluje się nad niepozycjonowanymi,
       więc bez tego kontenera gilosz przykryłby tekst. Ujemny z-index to pułapka -
       stary WebKit chowa wtedy obrazek pod tłem strony. */
    .cert-body {
      position: relative;
      z-index: 1;
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

    /* Na blankiecie nagłówek tabeli nie może mieć wypełnienia - szary pasek zakryłby
       gilosz pod spodem. Reguła dotyczy tylko odwrotu z tłem, więc wydruki bez wzoru
       zachowują dotychczasowy wygląd tabeli. */
    .cert-back th {
      background: transparent;
    }
  </style>
</head>
<body class="` + bodyClass + `">` + front + back + `</body>
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

	// Podpis pod kodem mówi, po co ten kwadrat tu jest - bez niego odbiorca dokumentu
	// nie wie, że to odnośnik do sprawdzenia ważności. Napis jest po polsku także na
	// wydrukach w innych językach, tak samo jak adnotacja DUPLIKAT: to opis elementu
	// polskiego dokumentu urzędowego.
	return `<span class="qr-code"><img src="` + dataURI + `" alt="Kod QR do weryfikacji zaświadczenia">` +
		`<span class="qr-caption">Sprawdź ważność</span></span>`
}

// guillocheHTML buduje warstwę tła. Obraz jest pozycjonowany bezwzględnie i wychodzi
// poza obszar treści, bo ma pokryć CAŁĄ stronę razem z marginesami - na blankiecie
// wzór sięga do krawędzi papieru.
func guillocheHTML(dataURI, modifier string) string {
	class := "cert-guilloche"
	if modifier != "" {
		class += " cert-guilloche--" + modifier
	}
	return `<img class="` + class + `" src="` + html.EscapeString(dataURI) + `" alt="">`
}

// wrapCertificateBack podkłada wzór pod odwrót zaświadczenia (tabelę programu).
// Odwrót blankietu też jest zadrukowany, więc strona z programem nie może być biała.
func wrapCertificateBack(back, dataURI string) string {
	if back == "" || dataURI == "" {
		return back
	}

	// Podział strony siedzi na samym kontenerze, a nie w osobnym pustym <div> przed nim:
	// tło jest pozycjonowane bezwzględnie względem kontenera, a przy podziale z zewnątrz
	// wylewało się na dół pierwszej strony.
	const pageBreak = `<div class="break"></div>`
	body := strings.TrimPrefix(strings.TrimSpace(back), pageBreak)

	return `<div class="cert-back">` + guillocheHTML(dataURI, "back") +
		`<div class="cert-body">` + body + `</div></div>`
}

// decorImageHTML buduje nadruk jako element inline. Nie <div>, bo znacznik bywa
// wstawiony w akapicie, a <div> w <p> parser HTML wyrzuca poza akapit - nadruk
// przestałby wtedy słuchać wyrównania ustawionego przez autora szablonu.
//
// Adres obrazu jest escapowany mimo że powstaje w tym pakiecie z bajtów spod stałego
// prefiksu: to jedyne miejsce, w którym wartość spoza szablonu trafia do atrybutu.
func decorImageHTML(class string, img decorImage, alt string) string {
	return `<span class="` + class + `" style="width:` + strconv.Itoa(img.WidthMM) + `mm">` +
		`<img src="` + html.EscapeString(img.DataURI) + `" alt="` + alt + `"></span>`
}

// buildCertificateRawValues zbiera nadruki wstawiane bez escapowania. Brakującego
// zasobu tu nie ma, więc jego znacznik zniknie z wydruku - tak samo jak {{ kod_qr }}
// przy niewykonfigurowanym adresie weryfikacji.
func buildCertificateRawValues(qrHTML string, decor certificateDecor) map[string]string {
	raw := map[string]string{}
	if qrHTML != "" {
		raw["kod_qr"] = qrHTML
	}
	for _, mark := range decorMarks(decor) {
		if mark.image.DataURI == "" {
			continue
		}
		raw[mark.key] = decorImageHTML(mark.class, mark.image, mark.alt)
	}
	return raw
}

// buildFallbackMarks buduje pasek u dołu strony z nadrukami, których autor szablonu
// nie umieścił znacznikiem. Dzięki temu nadruk pojawia się także na szablonach
// napisanych przed wprowadzeniem tej funkcji.
func buildFallbackMarks(decor certificateDecor, placed map[string]bool) string {
	var marks strings.Builder
	for _, mark := range decorMarks(decor) {
		if mark.image.DataURI == "" || placed[mark.key] {
			continue
		}
		marks.WriteString(decorImageHTML(mark.class, mark.image, mark.alt))
	}
	if marks.Len() == 0 {
		return ""
	}
	return `<div class="cert-marks">` + marks.String() + `</div>`
}

type decorMark struct {
	key   string
	class string
	alt   string
	image decorImage
}

// decorMarks trzyma kolejność nadruków na pasku i powiązanie rodzaju ze znacznikiem
// szablonu w jednym miejscu.
func decorMarks(decor certificateDecor) []decorMark {
	return []decorMark{
		{key: certassets.KindStampRound, class: "cert-stamp", alt: "Pieczątka okrągła", image: decor.StampRound},
		{key: certassets.KindStampCompany, class: "cert-stamp", alt: "Pieczątka firmowa", image: decor.StampCompany},
		{key: certassets.KindStampPersonal, class: "cert-stamp", alt: "Pieczątka imienna", image: decor.StampPersonal},
		{key: certassets.KindSignature, class: "cert-signature", alt: "Podpis", image: decor.Signature},
	}
}

// substituteCertificateTemplate podmienia znaczniki w szablonie kursu. Zwraca też
// zbiór znaczników, które faktycznie wystąpiły - wywołujący dokłada w rogu i w pasku
// u dołu tylko to, czego w szablonie nie było.
func substituteCertificateTemplate(certificate sqlc.GetCertificateByIDRow, rawValues map[string]string) (string, map[string]bool) {
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

	// rawValues omijają html.EscapeString, więc wolno tam wkładać WYŁĄCZNIE HTML zbudowany
	// w tym pakiecie z danych, których nie kontroluje użytkownik: kod QR wygenerowany
	// z kodu weryfikacyjnego oraz nadruki złożone ze stałego prefiksu data URI i bajtów
	// obrazu zakodowanych base64. Typ MIME jest stałą w kodzie, nie wartością z bazy.
	placed := map[string]bool{}
	substituted := certificatePlaceholderPattern.ReplaceAllStringFunc(certificate.CertFrontPage, func(token string) string {
		matches := certificatePlaceholderPattern.FindStringSubmatch(token)
		if len(matches) != 2 {
			return ""
		}

		normalized := strings.Join(strings.Fields(matches[1]), "")
		if raw, ok := rawValues[normalized]; ok {
			placed[normalized] = true
			return raw
		}
		return html.EscapeString(values[normalized])
	})

	return substituted, placed
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
