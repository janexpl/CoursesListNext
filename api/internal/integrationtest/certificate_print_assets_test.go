package integrationtest

import (
	"fmt"
	"net/http"
	"testing"
)

// Nadruki - pieczątki, podpis i gilosz - dostają wyłącznie zaświadczenia wystawione
// przez platformę, czyli te z kluczem idempotencji. Wydruki z aplikacji webowej idą
// na papier firmowy i są stemplowane w biurze.

type printDecorEnvelope struct {
	Data struct {
		PrintDecor *struct {
			StampRound *struct {
				URL     string `json:"url"`
				WidthMm int    `json:"widthMm"`
			} `json:"stampRound"`
			GuillocheFrontURL string `json:"guillocheFrontUrl"`
			GuillocheBackURL  string `json:"guillocheBackUrl"`
		} `json:"printDecor"`
	} `json:"data"`
}

func decodePrintDecor(t *testing.T, resp apiResponse) printDecorEnvelope {
	t.Helper()
	var body printDecorEnvelope
	resp.decode(t, &body)
	return body
}

func TestPrintDecorOnlyForPlatformCertificates(t *testing.T) {
	e := requireEnv(t)
	course := e.seedCourse(t)
	student := e.seedStudent(t)

	// Dokument platformowy: z nagłówkiem Idempotency-Key.
	payload := certificatePayload(student, course.ID)
	headers := map[string]string{"Idempotency-Key": fmt.Sprintf("it-decor-%d", nextSeed())}
	resp := e.mustCall(t, http.MethodPost, "/certificates", payload, headers)
	if resp.Status != http.StatusCreated {
		t.Fatalf("wystawienie przez platformę: %d %s", resp.Status, resp.Body)
	}
	platformID := decodeCreated(t, resp).ID

	// Dokument z aplikacji webowej: bez nagłówka.
	resp = e.mustCall(t, http.MethodPost, "/certificates", certificatePayload(e.seedStudent(t), course.ID), nil)
	if resp.Status != http.StatusCreated {
		t.Fatalf("wystawienie z aplikacji: %d %s", resp.Status, resp.Body)
	}
	webID := decodeCreated(t, resp).ID

	platform := decodePrintDecor(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", platformID), nil, nil))
	if platform.Data.PrintDecor == nil {
		t.Fatal("zaświadczenie platformowe powinno nieść printDecor")
	}
	if platform.Data.PrintDecor.GuillocheFrontURL == "" || platform.Data.PrintDecor.GuillocheBackURL == "" {
		t.Fatalf("brak adresów tła: %+v", platform.Data.PrintDecor)
	}

	web := decodePrintDecor(t, e.mustCall(t, http.MethodGet, fmt.Sprintf("/certificates/%d", webID), nil, nil))
	if web.Data.PrintDecor != nil {
		t.Fatalf("dokument spoza platformy nie może nieść printDecor: %+v", web.Data.PrintDecor)
	}
}

// Tło jest wkompilowane w API, więc trasa działa także wtedy, gdy nikt niczego nie wgrał.
func TestGuillochePatternIsServed(t *testing.T) {
	e := requireEnv(t)

	for _, path := range []string{
		"/certificate-print-assets/guilloche",
		"/certificate-print-assets/guilloche?side=back",
	} {
		resp := e.mustCall(t, http.MethodGet, path, nil, nil)
		if resp.Status != http.StatusOK {
			t.Fatalf("%s: %d %s", path, resp.Status, resp.Body)
		}
		if got := resp.Header.Get("Content-Type"); got != "image/png" {
			t.Fatalf("%s: Content-Type = %q", path, got)
		}
		if len(resp.Body) < 10<<10 {
			t.Fatalf("%s: wzór ma tylko %d bajtów", path, len(resp.Body))
		}
	}
}

// Pieczątki podmienia administrator z przeglądarki; klucz integracji nie może tego zrobić,
// bo decydują o wyglądzie każdego dokumentu wychodzącego z instytucji.
func TestPrintAssetUploadRejectsAPIKey(t *testing.T) {
	e := requireEnv(t)

	resp := e.mustCall(t, http.MethodDelete, "/admin/certificate-print-assets/pieczatka_okragla", nil, nil)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("oczekiwano 403, dostano %d: %s", resp.Status, resp.Body)
	}
	if got := resp.errorMessage(t); got != "this endpoint requires an interactive session" {
		t.Fatalf("odmowa ma wskazywać na wymóg sesji, dostano %q", got)
	}

	// Odczyt kluczem API jest w porządku - potrzebuje go podgląd zaświadczenia.
	if resp := e.mustCall(t, http.MethodGet, "/certificate-print-assets", nil, nil); resp.Status != http.StatusOK {
		t.Fatalf("lista nadruków: %d %s", resp.Status, resp.Body)
	}
}

func TestPrintAssetRejectsUnknownKind(t *testing.T) {
	e := requireEnv(t)

	resp := e.mustCall(t, http.MethodGet, "/certificate-print-assets/nieznany/file", nil, nil)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("oczekiwano 400, dostano %d: %s", resp.Status, resp.Body)
	}
}
