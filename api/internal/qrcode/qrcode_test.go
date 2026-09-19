package qrcode

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"

	"rsc.io/qr"
)

const dataURIPrefix = "data:image/png;base64,"

func decodePNG(t *testing.T, dataURI string) image.Image {
	t.Helper()
	if !strings.HasPrefix(dataURI, dataURIPrefix) {
		t.Fatalf("oczekiwano data URI z PNG, dostano %.40q", dataURI)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURI, dataURIPrefix))
	if err != nil {
		t.Fatalf("data URI nie jest poprawnym base64: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("zawartość nie jest poprawnym PNG: %v", err)
	}
	return decoded
}

func isBlack(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r < 0x8000 && g < 0x8000 && b < 0x8000
}

// Obraz musi odpowiadać matrycy kodu co do modułu - inaczej wydruk prowadziłby pod
// inny adres albo nie dałby się zeskanować.
func TestPNGDataURIDrawsEveryModuleOfTheCode(t *testing.T) {
	const text = "https://courseslist.example.pl/verify/K7QM4XPA9TZC"

	dataURI, err := PNGDataURI(text)
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}
	img := decodePNG(t, dataURI)

	code, err := qr.Encode(text, recoveryLevel)
	if err != nil {
		t.Fatalf("nie udało się zakodować wzorca porównawczego: %v", err)
	}

	modules := code.Size + 2*quietZoneModules
	scale := img.Bounds().Dx() / modules
	if img.Bounds().Dx() != img.Bounds().Dy() {
		t.Fatalf("obraz nie jest kwadratem: %v", img.Bounds())
	}
	if img.Bounds().Dx() != modules*scale {
		t.Fatalf("bok %d nie jest wielokrotnością liczby modułów %d", img.Bounds().Dx(), modules)
	}

	for y := range code.Size {
		for x := range code.Size {
			// Próbkujemy środek modułu, żeby test nie zależał od zaokrągleń na krawędziach.
			px := (x+quietZoneModules)*scale + scale/2
			py := (y+quietZoneModules)*scale + scale/2
			if got := isBlack(img, px, py); got != code.Black(x, y) {
				t.Fatalf("moduł (%d,%d): narysowano czarny=%v, w kodzie czarny=%v", x, y, got, code.Black(x, y))
			}
		}
	}
}

// Biała otulina (quiet zone) jest wymagana przez standard - bez niej czytniki gubią
// kod na tle dokumentu.
func TestPNGDataURIHasQuietZone(t *testing.T) {
	dataURI, err := PNGDataURI("https://courseslist.example.pl/verify/K7QM4XPA9TZC")
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}
	img := decodePNG(t, dataURI)
	bounds := img.Bounds()

	code, err := qr.Encode("https://courseslist.example.pl/verify/K7QM4XPA9TZC", recoveryLevel)
	if err != nil {
		t.Fatal(err)
	}
	scale := bounds.Dx() / (code.Size + 2*quietZoneModules)
	margin := quietZoneModules * scale

	for _, point := range [][2]int{
		{0, 0},
		{bounds.Dx() - 1, 0},
		{0, bounds.Dy() - 1},
		{bounds.Dx() - 1, bounds.Dy() - 1},
		{margin - 1, bounds.Dy() / 2},
		{bounds.Dx() / 2, margin - 1},
	} {
		if isBlack(img, point[0], point[1]) {
			t.Fatalf("otulina jest zamalowana w punkcie %v", point)
		}
	}
}

// Wydruk ma sens tylko wtedy, gdy obrazek ma dość pikseli na 24 mm papieru.
func TestPNGDataURIIsLargeEnoughForPrint(t *testing.T) {
	dataURI, err := PNGDataURI("https://courseslist.example.pl/verify/K7QM4XPA9TZC")
	if err != nil {
		t.Fatal(err)
	}
	if side := decodePNG(t, dataURI).Bounds().Dx(); side < 400 {
		t.Fatalf("bok %d px to za mało na druk (przy 24 mm daje poniżej 400 dpi)", side)
	}
}

func TestPNGDataURIIsDeterministic(t *testing.T) {
	first, err := PNGDataURI("https://courseslist.example.pl/verify/K7QM4XPA9TZC")
	if err != nil {
		t.Fatal(err)
	}
	second, err := PNGDataURI("https://courseslist.example.pl/verify/K7QM4XPA9TZC")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("ten sam adres dał dwa różne obrazy")
	}

	other, err := PNGDataURI("https://courseslist.example.pl/verify/AAAAAAAAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("różne adresy dały ten sam obraz")
	}
}

func TestPNGDataURIRejectsEmptyText(t *testing.T) {
	if _, err := PNGDataURI("   "); err == nil {
		t.Fatal("pusty tekst powinien być błędem")
	}
}

func TestVerificationURL(t *testing.T) {
	tests := []struct {
		name     string
		template string
		code     string
		want     string
	}{
		{
			name:     "znacznik w szablonie",
			template: "https://example.pl/verify/{code}",
			code:     "K7QM4XPA9TZC",
			want:     "https://example.pl/verify/K7QM4XPA9TZC",
		},
		{
			name:     "znacznik w środku adresu",
			template: "https://example.pl/z/{code}/sprawdz",
			code:     "K7QM4XPA9TZC",
			want:     "https://example.pl/z/K7QM4XPA9TZC/sprawdz",
		},
		{
			name:     "brak szablonu wyłącza QR",
			template: "   ",
			code:     "K7QM4XPA9TZC",
			want:     "",
		},
		{
			name:     "brak kodu wyłącza QR",
			template: "https://example.pl/verify/{code}",
			code:     "",
			want:     "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := VerificationURL(test.template, test.code); got != test.want {
				t.Fatalf("VerificationURL = %q, oczekiwano %q", got, test.want)
			}
		})
	}
}
