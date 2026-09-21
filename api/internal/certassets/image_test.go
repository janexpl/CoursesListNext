package certassets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// stampLike rysuje coś w rodzaju pieczątki: przezroczyste tło i okrągła ramka.
func stampLike(side int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	centre := float64(side) / 2
	radius := centre * 0.9
	for y := range side {
		for x := range side {
			dx := float64(x) - centre
			dy := float64(y) - centre
			distance := dx*dx + dy*dy
			if distance <= radius*radius && distance >= (radius*0.85)*(radius*0.85) {
				img.Set(x, y, color.NRGBA{R: 0x1D, G: 0x4E, B: 0xD8, A: 0xFF})
			}
		}
	}
	return img
}

// Skan pieczątki z telefonu ma kilka tysięcy pikseli boku. Bez przeskalowania każdy
// PDF niósłby kilka megabajtów obrazu, którego druk i tak nie odda.
func TestNormalizeShrinksOversizedScans(t *testing.T) {
	raw := encodePNG(t, stampLike(4000))

	normalized, width, height, err := Normalize(raw)
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}
	if width != MaxLongestSidePx || height != MaxLongestSidePx {
		t.Fatalf("po normalizacji %dx%d, oczekiwano %d na dłuższym boku", width, height, MaxLongestSidePx)
	}
	if len(normalized) > MaxNormalizedBytes {
		t.Fatalf("znormalizowany plik waży %d B, limit to %d B", len(normalized), MaxNormalizedBytes)
	}
	decoded, err := png.Decode(bytes.NewReader(normalized))
	if err != nil {
		t.Fatalf("wynik nie jest poprawnym PNG: %v", err)
	}
	if decoded.Bounds().Dx() != width {
		t.Fatalf("zadeklarowana szerokość %d nie zgadza się z obrazem %d", width, decoded.Bounds().Dx())
	}
}

// Mały obrazek zostaje w swoim rozmiarze - powiększanie tylko rozmyłoby pieczątkę.
func TestNormalizeKeepsSmallImages(t *testing.T) {
	raw := encodePNG(t, stampLike(300))

	_, width, height, err := Normalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if width != 300 || height != 300 {
		t.Fatalf("obraz zmienił rozmiar na %dx%d", width, height)
	}
}

// Przezroczystość jest tu wymaganiem, nie ozdobą: pieczątka na białym prostokącie
// zasłoniłaby gilosz pod spodem.
func TestNormalizeKeepsTransparency(t *testing.T) {
	raw := encodePNG(t, stampLike(1600))

	normalized, _, _, err := Normalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(normalized))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := decoded.At(1, 1).RGBA(); alpha != 0 {
		t.Fatalf("róg obrazu stracił przezroczystość (alfa %d)", alpha)
	}
}

// JPEG wchodzi, ale wychodzi zawsze PNG - wydruk wstawia stały prefiks data URI.
func TestNormalizeConvertsJPEGToPNG(t *testing.T) {
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, stampLike(800), nil); err != nil {
		t.Fatal(err)
	}

	normalized, _, _, err := Normalize(buffer.Bytes())
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(normalized)); err != nil {
		t.Fatalf("wynik nie jest PNG: %v", err)
	}
}

func TestNormalizeRejectsUnsupportedFormats(t *testing.T) {
	var animated bytes.Buffer
	if err := gif.Encode(&animated, stampLike(200), nil); err != nil {
		t.Fatal(err)
	}

	for name, raw := range map[string][]byte{
		"gif":        animated.Bytes(),
		"pdf":        []byte("%PDF-1.4\n%garbage"),
		"pusty plik": {},
		"śmieci":     []byte("to nie jest obraz"),
	} {
		if _, _, _, err := Normalize(raw); err == nil {
			t.Fatalf("%s: oczekiwano błędu", name)
		}
	}
}

// Nagłówek deklarujący gigantyczny obraz nie może doprowadzić do alokacji pamięci -
// dlatego rozmiar sprawdzamy przez DecodeConfig, przed pełnym dekodowaniem.
//
// Plik jest budowany ręcznie, a nie przez podmianę bajtów w gotowym PNG: podmiana
// psuje sumę kontrolną nagłówka i obraz odpadłby na niej, nie na strażniku rozmiaru,
// więc test przechodziłby z niewłaściwego powodu.
func TestNormalizeRejectsDeclaredHugeImages(t *testing.T) {
	header := make([]byte, 0, 13)
	header = binary.BigEndian.AppendUint32(header, 30000) // szerokość
	header = binary.BigEndian.AppendUint32(header, 30000) // wysokość
	header = append(header, 8, 6, 0, 0, 0)                // 8 bitów, RGBA, bez przeplotu

	chunk := append([]byte("IHDR"), header...)
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	png = binary.BigEndian.AppendUint32(png, uint32(len(header)))
	png = append(png, chunk...)
	png = binary.BigEndian.AppendUint32(png, crc32.ChecksumIEEE(chunk))

	_, _, _, err := Normalize(png)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("oczekiwano ErrImageTooLarge, dostano %v", err)
	}
}
