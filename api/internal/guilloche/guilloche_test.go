package guilloche

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"
)

func decode(t *testing.T, raw []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("wzór nie jest poprawnym PNG: %v", err)
	}
	return img
}

// Wzór jest tłem strony A4, więc jego proporcje muszą się zgadzać z arkuszem -
// inaczej ramka wyszłaby rozciągnięta w jedną stronę.
func TestPatternsMatchA4Proportions(t *testing.T) {
	for name, raw := range map[string][]byte{"przód": FrontPNG(), "odwrót": BackPNG()} {
		bounds := decode(t, raw).Bounds()
		ratio := float64(bounds.Dy()) / float64(bounds.Dx())
		const a4 = 297.0 / 210.0
		if math.Abs(ratio-a4) > 0.01 {
			t.Fatalf("%s: proporcje %.4f, oczekiwano %.4f (A4)", name, ratio, a4)
		}
	}
}

// Oba wzory jadą w każdym PDF-ie zaświadczenia platformowego, więc ich waga jest
// częścią wymagania. Budżet z zapasem wobec dzisiejszych ~300 kB na stronę.
func TestPatternsFitPrintBudget(t *testing.T) {
	for name, raw := range map[string][]byte{"przód": FrontPNG(), "odwrót": BackPNG()} {
		if len(raw) > 450<<10 {
			t.Fatalf("%s waży %d B, budżet to 450 kB", name, len(raw))
		}
	}
}

// Tło ma zdobić, a nie utrudniać czytanie. Sprawdzamy to liczbowo, bo wzór pochodzi
// ze skanu i przy kolejnej wymianie pliku łatwo o zbyt ciemną wersję.
func TestPatternsStayLightEnoughForText(t *testing.T) {
	for name, raw := range map[string][]byte{"przód": FrontPNG(), "odwrót": BackPNG()} {
		img := decode(t, raw)
		bounds := img.Bounds()

		var sum float64
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				sum += (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 0xFFFF
			}
		}

		if mean := sum / float64(bounds.Dx()*bounds.Dy()); mean < 0.80 {
			t.Fatalf("%s: średnia jasność %.3f - tekst na takim tle będzie się źle czytał", name, mean)
		}
	}
}

// Przód i odwrót muszą być różne: na przodzie jest monogram, na odwrocie nie.
func TestFrontAndBackDiffer(t *testing.T) {
	if bytes.Equal(FrontPNG(), BackPNG()) {
		t.Fatal("wzór przodu i odwrotu jest ten sam")
	}
}

func TestDataURIsWrapThePatterns(t *testing.T) {
	const prefix = "data:image/png;base64,"
	for name, pair := range map[string][2]any{
		"przód":  {FrontDataURI(), FrontPNG()},
		"odwrót": {BackDataURI(), BackPNG()},
	} {
		uri := pair[0].(string)
		raw := pair[1].([]byte)
		if !strings.HasPrefix(uri, prefix) {
			t.Fatalf("%s: oczekiwano data URI z PNG, dostano %.40q", name, uri)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
		if err != nil {
			t.Fatalf("%s: data URI nie jest poprawnym base64: %v", name, err)
		}
		if !bytes.Equal(decoded, raw) {
			t.Fatalf("%s: data URI niesie inny obraz niż plik", name)
		}
	}
}
