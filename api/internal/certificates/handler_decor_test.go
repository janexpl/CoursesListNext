package certificates

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/certassets"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

func printAssetRows() []sqlc.ListCertificatePrintAssetFilesRow {
	return []sqlc.ListCertificatePrintAssetFilesRow{
		{Kind: certassets.KindStampRound, ContentType: "image/png", FileData: []byte{0x01, 0x02}, PrintWidthMm: 35},
		{Kind: certassets.KindStampCompany, ContentType: "image/png", FileData: []byte{0x03, 0x04}, PrintWidthMm: 30},
		{Kind: certassets.KindStampPersonal, ContentType: "image/png", FileData: []byte{0x07, 0x08}, PrintWidthMm: 25},
		{Kind: certassets.KindSignature, ContentType: "image/png", FileData: []byte{0x05, 0x06}, PrintWidthMm: 50},
	}
}

func platformCertificate() sqlc.GetCertificateByIDRow {
	certificate := baseCertificateForPDF()
	certificate.IdempotencyKey = pgtype.Text{String: "platforma-123", Valid: true}
	return certificate
}

// Nadruki dostaje wyłącznie dokument wystawiony przez platformę. Wydruki z aplikacji
// webowej i z dziennika idą na papier firmowy i są stemplowane w biurze.
func TestLoadCertificateDecorOnlyForPlatformCertificates(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listPrintAssetFilesFunc: func(ctx context.Context) ([]sqlc.ListCertificatePrintAssetFilesRow, error) {
			return printAssetRows(), nil
		},
	}, nil)

	withKey := handler.loadCertificateDecor(t.Context(), platformCertificate())
	if withKey.StampRound.DataURI == "" || withKey.StampCompany.DataURI == "" || withKey.StampPersonal.DataURI == "" || withKey.Signature.DataURI == "" {
		t.Fatalf("zaświadczenie platformowe powinno dostać wszystkie nadruki: %+v", withKey)
	}
	if withKey.GuillocheFront == "" {
		t.Fatal("zaświadczenie platformowe powinno dostać tło giloszowe")
	}
	if withKey.StampCompany.WidthMM != 30 {
		t.Fatalf("szerokość nadruku ma pochodzić z bazy, dostano %d", withKey.StampCompany.WidthMM)
	}

	withoutKey := handler.loadCertificateDecor(t.Context(), baseCertificateForPDF())
	if withoutKey != (certificateDecor{}) {
		t.Fatalf("dokument spoza platformy nie może dostać nadruków: %+v", withoutKey)
	}
}

// Pusty klucz idempotencji to nadal dokument spoza platformy - kolumna bywa pustym
// tekstem, nie tylko NULL-em.
func TestLoadCertificateDecorIgnoresEmptyIdempotencyKey(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listPrintAssetFilesFunc: func(ctx context.Context) ([]sqlc.ListCertificatePrintAssetFilesRow, error) {
			return printAssetRows(), nil
		},
	}, nil)

	certificate := baseCertificateForPDF()
	certificate.IdempotencyKey = pgtype.Text{String: "", Valid: true}

	if decor := handler.loadCertificateDecor(t.Context(), certificate); decor != (certificateDecor{}) {
		t.Fatalf("pusty klucz nie czyni dokumentu platformowym: %+v", decor)
	}
}

// Awaria odczytu nie może pozbawić kursanta dokumentu - wydruk bez pieczątki jest
// lepszy niż błąd 500, tak samo jak przy kodzie QR.
func TestLoadCertificateDecorSurvivesDatabaseError(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listPrintAssetFilesFunc: func(ctx context.Context) ([]sqlc.ListCertificatePrintAssetFilesRow, error) {
			return nil, errors.New("baza nie odpowiada")
		},
	}, nil)

	if decor := handler.loadCertificateDecor(t.Context(), platformCertificate()); decor != (certificateDecor{}) {
		t.Fatalf("po błędzie odczytu wydruk ma iść bez nadruków: %+v", decor)
	}
}

// Dopóki administrator nic nie wgrał, platformowy wydruk dostaje samo tło.
func TestLoadCertificateDecorWithoutUploadedFiles(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listPrintAssetFilesFunc: func(ctx context.Context) ([]sqlc.ListCertificatePrintAssetFilesRow, error) {
			return nil, nil
		},
	}, nil)

	decor := handler.loadCertificateDecor(t.Context(), platformCertificate())
	if decor.StampRound.DataURI != "" || decor.Signature.DataURI != "" {
		t.Fatalf("bez wgranych plików nie ma pieczątek: %+v", decor)
	}
	if decor.GuillocheFront == "" {
		t.Fatal("tło powstaje w kodzie, więc nie zależy od wgranych plików")
	}
}
