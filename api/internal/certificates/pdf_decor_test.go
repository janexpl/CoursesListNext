package certificates

import (
	"strings"
	"testing"
)

const (
	testStampURI     = "data:image/png;base64,AAAASTAMPROUND"
	testStamp2URI    = "data:image/png;base64,AAAASTAMPCOMPANY"
	testStamp3URI    = "data:image/png;base64,AAAASTAMPPERSONAL"
	testSignatureURI = "data:image/png;base64,AAAASIGN"
	testGuillocheURI = "data:image/png;base64,AAAAGUILLOCHE"
)

func testDecor() certificateDecor {
	return certificateDecor{
		StampRound:     decorImage{DataURI: testStampURI, WidthMM: 30},
		StampCompany:   decorImage{DataURI: testStamp2URI, WidthMM: 40},
		StampPersonal:  decorImage{DataURI: testStamp3URI, WidthMM: 35},
		Signature:      decorImage{DataURI: testSignatureURI, WidthMM: 45},
		GuillocheFront: testGuillocheURI,
	}
}

// Nadruki dostają wyłącznie zaświadczenia platformowe. Dla pozostałych 16 tysięcy
// dokumentów wydruk ma zostać bajt w bajt taki, jak przed tą zmianą.
func TestCertificateWithoutDecorPrintsExactlyAsBefore(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"

	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate, certificateDecor{})

	for _, marker := range []string{"cert-guilloche", "cert-marks", "cert-front--marks", "cert-stamp", "cert-signature"} {
		if strings.Contains(html, `class="`+marker) {
			t.Fatalf("wydruk bez nadruków zawiera %q", marker)
		}
	}
	// Kod QR działa jak dotąd.
	if !strings.Contains(html, `<div class="qr-corner">`) {
		t.Fatal("brak kodu QR w rogu - zmiana zepsuła dotychczasowe zachowanie")
	}
}

func TestDecorLandsInTemplateMarkers(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"
	certificate.CertFrontPage = `<p>{{ pieczatka_okragla }} {{ pieczatka_firmowa }} {{ pieczatka_imienna }}</p><p>{{ podpis }}</p>`

	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate, testDecor())

	for _, uri := range []string{testStampURI, testStamp2URI, testStamp3URI, testSignatureURI} {
		if !strings.Contains(html, uri) {
			t.Fatalf("w wydruku brakuje obrazu %q", uri)
		}
	}
	if strings.Contains(html, `<div class="cert-marks">`) {
		t.Fatal("nadruki wstawione znacznikami nie mogą dodatkowo lądować w pasku awaryjnym")
	}
	// Asercja celuje w atrybut class, a nie w samo słowo: reguła .cert-front--marks
	// stoi w arkuszu stylów zawsze, więc szukanie jej w całym dokumencie zawsze trafia.
	if strings.Contains(html, `class="cert-front cert-front--marks"`) {
		t.Fatal("bez paska awaryjnego strona nie potrzebuje niższej treści")
	}
	// Element musi być inline, bo znacznik bywa w akapicie - <div> w <p> parser wyrzuca
	// poza akapit i nadruk przestaje słuchać wyrównania z szablonu.
	if !strings.Contains(html, `<span class="cert-stamp"`) {
		t.Fatalf("pieczątka musi być elementem inline: %s", html)
	}
}

// Szablony istniejących kursów nie mają znaczników, a nadruk ma się na nich pojawić.
func TestDecorFallsBackToBottomStrip(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"

	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate, testDecor())

	if !strings.Contains(html, `<div class="cert-marks">`) {
		t.Fatal("brak paska awaryjnego z nadrukami")
	}
	if !strings.Contains(html, "cert-front--marks") {
		t.Fatal("pasek zabiera miejsce treści, więc kontener musi dostać modyfikator")
	}
	if !strings.Contains(html, `<div class="qr-corner">`) {
		t.Fatal("pasek nie może wyprzeć kodu QR z rogu")
	}
}

// Autor szablonu może wskazać miejsce tylko dla części nadruków.
func TestDecorMixesMarkersWithStrip(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.VerificationCode = "K7QM4XPA9TZC"
	certificate.CertFrontPage = `<p style="text-align:right">{{ podpis }}</p>`

	html := buildCertificatePDFHTML(certificate, testVerificationURLTemplate, testDecor())

	marks := html[strings.Index(html, `<div class="cert-marks">`):]
	for _, uri := range []string{testStampURI, testStamp2URI, testStamp3URI} {
		if !strings.Contains(marks, uri) {
			t.Fatalf("pieczątka bez znacznika powinna trafić do paska: %q", uri)
		}
	}
	if strings.Contains(marks, testSignatureURI) {
		t.Fatal("podpis wstawiony znacznikiem nie może się powtórzyć w pasku")
	}
}

// Dopóki administrator nie wgra pliku, znacznik ma zniknąć bez śladu - tak jak
// {{ kod_qr }} przy niewykonfigurowanym adresie weryfikacji.
func TestMissingAssetRemovesItsMarker(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.CertFrontPage = `<p>{{ pieczatka_okragla }}</p>`

	html := buildCertificatePDFHTML(certificate, "", certificateDecor{
		Signature: decorImage{DataURI: testSignatureURI, WidthMM: 45},
	})

	if strings.Contains(html, "pieczatka_okragla") {
		t.Fatalf("znacznik bez wgranego pliku został w wydruku: %s", html)
	}
	if strings.Contains(html, `<span class="cert-stamp"`) {
		t.Fatal("brak pliku nie może zostawiać pustego elementu")
	}
}

// Gilosz musi być elementem treści, a nie tłem CSS - tła bywają pomijane przy druku.
// Treść musi leżeć nad nim, inaczej deseń przykryłby tekst.
func TestGuillocheIsContentBehindTheText(t *testing.T) {
	certificate := baseCertificateForPDF()

	html := buildCertificatePDFHTML(certificate, "", certificateDecor{GuillocheFront: testGuillocheURI})

	if !strings.Contains(html, `<img class="cert-guilloche" src="`+testGuillocheURI+`"`) {
		t.Fatalf("gilosz musi być obrazem w treści: %s", html)
	}
	// Deseń nie może trafić do CSS jako tło - tła bywają pomijane przy druku.
	// Szukamy użycia w url(), bo samo słowo "background-image" pada w komentarzu arkusza.
	if strings.Contains(html, "url("+testGuillocheURI) {
		t.Fatal("gilosz nie może być tłem CSS")
	}
	if !strings.Contains(html, `<div class="cert-body">`) {
		t.Fatal("treść musi mieć własny pozycjonowany kontener, żeby nie zniknęła pod deseniem")
	}
	if strings.Contains(html, "z-index: -1") {
		t.Fatal("ujemny z-index chowa deseń pod tłem strony w starym WebKicie")
	}
}

// Regresja bezpieczeństwa: nadruki omijają html.EscapeString, więc do HTML nie może
// trafić nic, co pochodzi spoza tego pakietu.
func TestDecorCannotInjectMarkup(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.CertFrontPage = `<p>{{ pieczatka_okragla }}</p>`

	html := buildCertificatePDFHTML(certificate, "", certificateDecor{
		StampRound: decorImage{DataURI: `data:image/png;base64,AAA"><script>alert(1)</script>`, WidthMM: 35},
	})

	if strings.Contains(html, "<script>") {
		t.Fatalf("wstrzyknięty znacznik trafił do wydruku: %s", html)
	}
}
