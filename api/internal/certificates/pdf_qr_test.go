package certificates

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"regexp"
	"strings"
	"testing"

	"rsc.io/qr"
)

const testVerificationURLTemplate = "https://courseslist.example.pl/verify/{code}"

var pdfQRImagePattern = regexp.MustCompile(`data:image/png;base64,([A-Za-z0-9+/=]+)`)

// decodeQRModules odczytuje z wydruku obraz kodu QR i zwraca go jako siatkę modułów.
func decodeQRModules(t *testing.T, html string) [][]bool {
	t.Helper()
	matches := pdfQRImagePattern.FindStringSubmatch(html)
	if matches == nil {
		t.Fatalf("w wydruku nie ma obrazka kodu QR")
	}
	raw, err := base64.StdEncoding.DecodeString(matches[1])
	if err != nil {
		t.Fatalf("obrazek nie jest poprawnym base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("obrazek nie jest poprawnym PNG: %v", err)
	}

	// Rozmiar modułu odczytujemy z otuliny: pierwszy czarny piksel na przekątnej
	// wypada na początku wzorca pozycjonującego, czyli po 4 modułach otuliny.
	side := img.Bounds().Dx()
	firstBlack := -1
	for i := range side {
		if r, _, _, _ := img.At(i, i).RGBA(); r < 0x8000 {
			firstBlack = i
			break
		}
	}
	if firstBlack <= 0 {
		t.Fatal("nie znaleziono wzorca pozycjonującego - to nie wygląda na kod QR")
	}
	scale := firstBlack / 4
	modules := side / scale

	grid := make([][]bool, modules)
	for y := range grid {
		grid[y] = make([]bool, modules)
		for x := range grid[y] {
			r, _, _, _ := img.At(x*scale+scale/2, y*scale+scale/2).RGBA()
			grid[y][x] = r < 0x8000
		}
	}
	return grid
}

// assertQREncodes sprawdza, że narysowany kod niesie dokładnie oczekiwany adres.
// Porównujemy z kodem zakodowanym przez tę samą bibliotekę - kodowanie jest
// deterministyczne, więc każda różnica w treści zmienia siatkę modułów.
func assertQREncodes(t *testing.T, html, wantURL string) {
	t.Helper()
	drawn := decodeQRModules(t, html)

	expected, err := qr.Encode(wantURL, qr.Q)
	if err != nil {
		t.Fatalf("nie udało się zakodować wzorca porównawczego: %v", err)
	}
	if got, want := len(drawn), expected.Size+8; got != want {
		t.Fatalf("kod ma %d modułów na bok, oczekiwano %d (inna treść albo inne ustawienia)", got, want)
	}
	for y := range expected.Size {
		for x := range expected.Size {
			if drawn[y+4][x+4] != expected.Black(x, y) {
				t.Fatalf("moduł (%d,%d) różni się od kodu dla %q", x, y, wantURL)
			}
		}
	}
}

func TestCertificatePDFPrintsQRInPlaceOfPlaceholder(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"
	certificate.CertFrontPage = `<h1>ZAŚWIADCZENIE</h1><p>{{ numer_zaswiadczenia }}</p><div>{{ kod_qr }}</div>`

	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate)

	assertQREncodes(t, html, "https://courseslist.example.pl/verify/K7QM4XPA9TZC")
	if strings.Contains(html, `<div class="qr-corner">`) {
		t.Fatal("kod wstawiony znacznikiem nie może dodatkowo lądować w rogu")
	}
	if n := strings.Count(html, "data:image/png;base64,"); n != 1 {
		t.Fatalf("oczekiwano jednego kodu QR, znaleziono %d", n)
	}
}

// Szablony 132 istniejących kursów nie mają znacznika, a QR ma się na nich pojawić.
func TestCertificatePDFPrintsQRInTheCornerWithoutPlaceholder(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"

	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate)

	assertQREncodes(t, html, "https://courseslist.example.pl/verify/K7QM4XPA9TZC")
	if !strings.Contains(html, `<div class="qr-corner">`) {
		t.Fatalf("brak bloku narożnego w wydruku")
	}
	if n := strings.Count(html, "data:image/png;base64,"); n != 1 {
		t.Fatalf("oczekiwano jednego kodu QR, znaleziono %d", n)
	}
}

func TestCertificatePDFHasNoQRWithoutConfiguredURL(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"
	certificate.CertFrontPage = `<h1>ZAŚWIADCZENIE</h1><div>{{ kod_qr }}</div>`

	html := buildCertificatePDFHTML(certificate, "")

	if strings.Contains(html, "data:image/png;base64,") || strings.Contains(html, `<div class="qr-corner">`) {
		t.Fatal("bez skonfigurowanego adresu wydruk nie może zawierać kodu QR")
	}
	// Nieobsłużony znacznik znika, jak każdy inny nieznany - nie zostaje w treści.
	if strings.Contains(html, "kod_qr") {
		t.Fatalf("znacznik został w wydruku: %s", html)
	}
}

func TestCertificatePDFQRPlaceholderIgnoresWhitespace(t *testing.T) {
	for _, token := range []string{"{{kod_qr}}", "{{ kod_qr }}", "{{   kod_qr   }}"} {
		certificate := baseCertificateForPDF()
		certificate.VerificationCode = "K7QM4XPA9TZC"
		certificate.CertFrontPage = `<div>` + token + `</div>`

		html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate)
		if strings.Contains(html, `<div class="qr-corner">`) {
			t.Fatalf("token %q nie został rozpoznany", token)
		}
	}

	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"
	certificate.CertFrontPage = `<div>{{ kod_qrx }}</div>`
	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate)
	if !strings.Contains(html, `<div class="qr-corner">`) {
		t.Fatal("nieznany token nie może uchodzić za znacznik kodu QR")
	}
}

// Regresja: wprowadzenie surowego HTML dla kodu QR nie może otworzyć drogi
// dla niezaescapowanych danych z bazy.
func TestCertificatePDFStillEscapesTemplateValues(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.StudentLastname = `<script>alert(1)</script>`
	certificate.CertFrontPage = `<p>{{ nazwisko }}</p>`

	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatal("wartość z bazy trafiła do wydruku bez escapowania")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("oczekiwano zaescapowanej wartości: %s", html)
	}
}
