package guilloche

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
)

func decode(t *testing.T) image.Image {
	t.Helper()
	raw, err := PNG()
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("wynik nie jest poprawnym PNG: %v", err)
	}
	return img
}

// Deseń musi być identyczny na każdym dokumencie - inaczej przeglądarka nie mogłaby
// trzymać go w cache, a każde zaświadczenie niosłoby własną kopię.
func TestPNGIsDeterministic(t *testing.T) {
	first, err := PNG()
	if err != nil {
		t.Fatal(err)
	}
	second, err := PNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("dwa wywołania dały różne obrazy")
	}
}

// Tło wędruje w każdym PDF-ie zaświadczenia platformowego, więc jego waga jest
// częścią wymagania, a nie szczegółem implementacji.
func TestPNGFitsPrintBudget(t *testing.T) {
	raw, err := PNG()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 200<<10 {
		t.Fatalf("deseń waży %d B, budżet to 200 kB", len(raw))
	}
}

// Gilosz ma zdobić, a nie przeszkadzać w czytaniu. Sprawdzamy to liczbowo: średnia
// jasność całej kartki i brak pikseli ciemniejszych niż najciemniejszy odcień palety.
func TestPatternStaysLight(t *testing.T) {
	img := decode(t)
	bounds := img.Bounds()

	darkest := uint32(0xFFFF)
	var sum float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			luminance := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 0xFFFF
			sum += luminance
			if r < darkest {
				darkest = r
			}
		}
	}

	mean := sum / float64(bounds.Dx()*bounds.Dy())
	if mean < 0.93 {
		t.Fatalf("średnia jasność %.4f - deseń przyciemnia kartkę o więcej niż 7%%", mean)
	}
	// Najciemniejszy odcień palety ma luminancję ok. 0,85; tekst zaświadczenia to #0f172a.
	if got := float64(darkest) / 0xFFFF; got < 0.78 {
		t.Fatalf("najciemniejszy piksel ma jasność %.3f - za ciemny jak na tło pod tekstem", got)
	}
}

// Biały margines to miejsce na nieprecyzyjne podawanie papieru przez drukarkę.
func TestPatternKeepsWhiteMargin(t *testing.T) {
	img := decode(t)
	bounds := img.Bounds()
	margin := marginPixels(bounds.Dx())

	isWhite := func(x, y int) bool {
		r, g, b, _ := img.At(x, y).RGBA()
		return r == 0xFFFF && g == 0xFFFF && b == 0xFFFF
	}

	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for _, y := range []int{bounds.Min.Y, bounds.Min.Y + margin - 1, bounds.Max.Y - margin, bounds.Max.Y - 1} {
			if !isWhite(x, y) {
				t.Fatalf("margines zamalowany w punkcie (%d,%d)", x, y)
			}
		}
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for _, x := range []int{bounds.Min.X, bounds.Min.X + margin - 1, bounds.Max.X - margin, bounds.Max.X - 1} {
			if !isWhite(x, y) {
				t.Fatalf("margines zamalowany w punkcie (%d,%d)", x, y)
			}
		}
	}
}

// Deseń musi faktycznie coś narysować - test jasności przeszedłby też dla pustej kartki.
func TestPatternDrawsLines(t *testing.T) {
	img := decode(t)
	bounds := img.Bounds()

	inked := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0xFFFF {
				inked++
			}
		}
	}

	share := float64(inked) / float64(bounds.Dx()*bounds.Dy())
	if share < 0.02 {
		t.Fatalf("zamalowano tylko %.3f%% powierzchni - to nie wygląda na gilosz", share*100)
	}
	if share > 0.25 {
		t.Fatalf("zamalowano %.1f%% powierzchni - deseń jest za gęsty pod tekstem", share*100)
	}
}

// Regresja: rozeta biegunowa potrafi zdegenerować się przy środku obrazu w plamę
// z kropek - akurat tam, gdzie na zaświadczeniu wypada tekst.
func TestPatternHasNoBlobInTheMiddle(t *testing.T) {
	img := decode(t)
	bounds := img.Bounds()

	inkShare := func(x0, y0, x1, y1 int) float64 {
		inked := 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if r, _, _, _ := img.At(x, y).RGBA(); r < 0xFFFF {
					inked++
				}
			}
		}
		return float64(inked) / float64((x1-x0)*(y1-y0))
	}

	margin := marginPixels(bounds.Dx())
	overall := inkShare(margin, margin, bounds.Dx()-margin, bounds.Dy()-margin)
	centre := inkShare(
		bounds.Dx()*2/5, bounds.Dy()*2/5,
		bounds.Dx()*3/5, bounds.Dy()*3/5,
	)

	if centre > overall {
		t.Fatalf("środek jest gęstszy (%.3f) niż reszta kartki (%.3f) - rozeta się zbiega", centre, overall)
	}
}

func TestDataURIWrapsThePNG(t *testing.T) {
	uri, err := DataURI()
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("oczekiwano data URI z PNG, dostano %.40q", uri)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatalf("data URI nie jest poprawnym base64: %v", err)
	}
	expected, err := PNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, expected) {
		t.Fatal("data URI niesie inny obraz niż PNG()")
	}
}
