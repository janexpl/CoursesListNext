package certificates

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

func baseCertificateForPDF() sqlc.GetCertificateByIDRow {
	return sqlc.GetCertificateByIDRow{
		ID:               7,
		Date:             pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true},
		StudentFirstname: "Jan",
		StudentLastname:  "Nowak",
		StudentBirthdate: pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
		CourseName:       "Szkolenie BHP",
		CourseSymbol:     "BHP",
		RegistryYear:     2026,
		RegistryNumber:   12,
		CourseDateStart:  pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true},
		CertFrontPage:    "<h1>ZAŚWIADCZENIE</h1><p>{{ numer_zaswiadczenia }}</p>",
		LanguageCode:     "pl",
	}
}

func TestCertificatePDFHasNoDuplicateAnnotationByDefault(t *testing.T) {
	html := buildCertificatePDFHTML(baseCertificateForPDF())

	if strings.Contains(html, "DUPLIKAT") {
		t.Fatal("zwykłe zaświadczenie nie może mieć adnotacji duplikatu")
	}
}

// Duplikat to ten sam dokument - na wydruku odróżnia go adnotacja z datą wystawienia.
func TestCertificatePDFCarriesDuplicateAnnotation(t *testing.T) {
	certificate := baseCertificateForPDF()
	certificate.DuplicateIssuedAt = pgtype.Timestamptz{
		Time:  time.Date(2026, time.October, 2, 9, 30, 0, 0, time.Local),
		Valid: true,
	}

	html := buildCertificatePDFHTML(certificate)

	if !strings.Contains(html, "DUPLIKAT") {
		t.Fatalf("brak adnotacji DUPLIKAT w wydruku: %s", html)
	}
	if !strings.Contains(html, "Data wystawienia duplikatu: 02.10.2026") {
		t.Fatalf("brak daty wystawienia duplikatu w wydruku: %s", html)
	}
	// Numer rejestru zostaje ten sam co na oryginale.
	if !strings.Contains(html, "12/BHP/2026") {
		t.Fatalf("duplikat musi nieść numer oryginału: %s", html)
	}
}
