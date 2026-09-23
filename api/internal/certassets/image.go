package certassets

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"

	// Rejestracja dekoderów dla formatów przyjmowanych od administratora.
	_ "image/jpeg"

	"golang.org/x/image/draw"
)

const (
	// MaxUploadBytes to górna granica wielkości wgrywanego pliku - tyle samo,
	// co przy skanach dzienników.
	MaxUploadBytes = 16 << 20

	// MaxSourcePixels chroni przed bombą dekompresyjną: mały plik może deklarować
	// obraz o ogromnych wymiarach, którego dekodowanie zjadłoby pamięć serwera.
	MaxSourcePixels = 8000

	// MaxLongestSidePx wyznacza rozdzielczość, od której zaczyna normalizacja.
	// Przy nadruku szerokim na 40 mm to ponad 600 dpi - więcej, niż odda jakakolwiek
	// drukarka biurowa.
	MaxLongestSidePx = 1000

	// MinLongestSidePx to granica, poniżej której nie schodzimy nawet po to, żeby
	// zmieścić się w budżecie. Przy 40 mm daje jeszcze ok. 250 dpi.
	MinLongestSidePx = 400

	// MaxNormalizedBytes pilnuje, żeby pojedynczy nadruk nie rozdął PDF-a. Cztery
	// nadruki plus tło giloszowe jadą w każdym wydruku platformowym.
	MaxNormalizedBytes = 250 << 10
)

var (
	// ErrUnsupportedImage - plik nie jest obrazem w obsługiwanym formacie.
	ErrUnsupportedImage = errors.New("certassets: unsupported image")

	// ErrImageTooLarge - obraz deklaruje wymiary, których nie zamierzamy dekodować.
	ErrImageTooLarge = errors.New("certassets: image too large")
)

// Normalize sprowadza wgrany plik do postaci, w jakiej trafia na wydruk: PNG
// o dłuższym boku najwyżej MaxLongestSidePx i wadze mieszczącej się w budżecie.
//
// Skan pieczątki z przezroczystym tłem potrafi ważyć blisko megabajt przy 1200 px,
// bo miękkie krawędzie i szum w kanale alfa kiepsko się pakują. Zamiast odrzucać taki
// plik, schodzimy z rozdzielczością, aż wynik zmieści się w budżecie - nadruk ma na
// papierze 30-45 mm, więc nawet 500 px to wciąż ponad 300 dpi.
//
// Wyjściem jest zawsze PNG, niezależnie od wejścia. Po pierwsze, przezroczystość
// pieczątki ma przetrwać - skan na białym tle zasłoniłby gilosz. Po drugie, wydruk
// wstawia obrazy z prefiksem "data:image/png;base64," jako stałą w kodzie, więc typ
// nie może zależeć od tego, co wgrał administrator.
//
// Skalowanie dzieje się przy wgrywaniu, a nie przy druku: plik wgrywa się raz,
// a drukuje tysiące razy.
func Normalize(raw []byte) (normalized []byte, width, height int, err error) {
	if len(raw) == 0 {
		return nil, 0, 0, ErrUnsupportedImage
	}

	// Wymiary czytamy z nagłówka przed pełnym dekodowaniem - inaczej plik deklarujący
	// 30000 x 30000 pikseli zaalokowałby kilka gigabajtów, zanim zdążylibyśmy go odrzucić.
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrUnsupportedImage, err)
	}
	if format != "png" && format != "jpeg" {
		return nil, 0, 0, fmt.Errorf("%w: %s", ErrUnsupportedImage, format)
	}
	if config.Width > MaxSourcePixels || config.Height > MaxSourcePixels {
		return nil, 0, 0, ErrImageTooLarge
	}

	source, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrUnsupportedImage, err)
	}

	longest := max(source.Bounds().Dx(), source.Bounds().Dy())
	side := min(longest, MaxLongestSidePx)

	for {
		target := scale(source, side)
		bounds := target.Bounds()

		var buffer bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buffer, target); err != nil {
			return nil, 0, 0, err
		}
		if buffer.Len() <= MaxNormalizedBytes || side <= MinLongestSidePx {
			if buffer.Len() > MaxNormalizedBytes {
				// Nawet w najmniejszej dopuszczalnej rozdzielczości plik nie mieści się
				// w budżecie - to już nie jest pieczątka, tylko zdjęcie.
				return nil, 0, 0, ErrImageTooLarge
			}
			return buffer.Bytes(), bounds.Dx(), bounds.Dy(), nil
		}

		// Krok w dół o jedną piątą: dość duży, żeby nie kodować obrazu kilkanaście razy,
		// dość mały, żeby nie zejść poniżej potrzebnej rozdzielczości.
		side = max(side*4/5, MinLongestSidePx)
	}
}

// scale zmniejsza obraz tak, by dłuższy bok miał zadaną długość. Obrazów mniejszych
// nie powiększamy - rozciągnięta pieczątka byłaby tylko rozmyta.
func scale(source image.Image, side int) image.Image {
	bounds := source.Bounds()
	longest := max(bounds.Dx(), bounds.Dy())
	if longest <= side {
		return source
	}

	ratio := float64(side) / float64(longest)
	width := max(int(float64(bounds.Dx())*ratio), 1)
	height := max(int(float64(bounds.Dy())*ratio), 1)

	target := image.NewNRGBA(image.Rect(0, 0, width, height))
	// CatmullRom, bo krawędź okrągłej pieczątki przy prostszych filtrach wychodzi schodkowa.
	draw.CatmullRom.Scale(target, target.Bounds(), source, bounds, draw.Over, nil)
	return target
}
