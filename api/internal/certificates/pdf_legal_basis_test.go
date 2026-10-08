package certificates

import (
	"strings"
	"testing"
)

func TestLegalBasisMarkerIsReplacedWithFrozenContent(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.CertFrontPage = `<p>Zaświadczenie wydano na podstawie {{ podstawa_prawna }}</p>`
	certificate.LegalBasis = "§ 16 ust. 3 rozporządzenia Ministra Gospodarki i Pracy z dnia 27 lipca 2004 r."

	html := buildCertificatePDFHTML(certificate, "", certificateDecor{})

	if !strings.Contains(html, "na podstawie § 16 ust. 3 rozporządzenia Ministra Gospodarki i Pracy z dnia 27 lipca 2004 r.") {
		t.Fatalf("podstawa prawna nie trafiła w miejsce znacznika: %s", html)
	}
	if strings.Contains(html, "podstawa_prawna") {
		t.Fatal("znacznik został w treści wydruku")
	}
}

// Treść podstawy wpisuje użytkownik w bibliotece, więc musi przejść przez escapowanie -
// w przeciwieństwie do kodu QR i nadruków budowanych w tym pakiecie.
func TestLegalBasisCannotInjectMarkup(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.CertFrontPage = `<p>{{ podstawa_prawna }}</p>`
	certificate.LegalBasis = `§ 1 <script>alert(1)</script>`

	html := buildCertificatePDFHTML(certificate, "", certificateDecor{})

	if strings.Contains(html, "<script>") {
		t.Fatalf("znacznik z treści podstawy trafił do wydruku: %s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("treść podstawy powinna zostać wydrukowana jako tekst")
	}
}

// Zapis znacznika bywa zmieniony przez edytor szablonu - jak przy pozostałych znacznikach.
func TestLegalBasisMarkerToleratesEditorMangling(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.CertFrontPage = `<p>{{ <span style="font-size:16px">Podstawa_Prawna</span> }}</p>`
	certificate.LegalBasis = "§ 18 ust. 2"

	html := buildCertificatePDFHTML(certificate, "", certificateDecor{})

	if !strings.Contains(html, "§ 18 ust. 2") {
		t.Fatalf("znacznik nie został rozpoznany: %s", html)
	}
}

// Kurs bez podstawy: znacznik znika bez śladu, jak każdy znacznik bez wartości.
func TestLegalBasisMarkerWithoutBasisLeavesNoTrace(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.CertFrontPage = `<p>na podstawie {{ podstawa_prawna }}.</p>`
	certificate.LegalBasis = ""

	html := buildCertificatePDFHTML(certificate, "", certificateDecor{})

	if strings.Contains(html, "podstawa_prawna") || strings.Contains(html, "{{") {
		t.Fatalf("znacznik bez podstawy został w treści: %s", html)
	}
}
