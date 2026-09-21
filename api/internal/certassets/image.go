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

	// MaxLongestSidePx wyznacza rozdzielczość nadruku. Przy szerokości 35 mm daje to
	// ponad 800 dpi, czyli więcej, niż odda jakakolwiek drukarka biurowa.
	MaxLongestSidePx = 1200

	// MaxNormalizedBytes pilnuje, żeby pojedynczy nadruk nie rozdął PDF-a.
	MaxNormalizedBytes = 400 << 10
)

var (
	// ErrUnsupportedImage - plik nie jest obrazem w obsługiwanym formacie.
	ErrUnsupportedImage = errors.New("certassets: unsupported image")

	// ErrImageTooLarge - obraz deklaruje wymiary, których nie zamierzamy dekodować.
	ErrImageTooLarge = errors.New("certassets: image too large")
)

// Normalize sprowadza wgrany plik do postaci, w jakiej trafia na wydruk: PNG
// o dłuższym boku najwyżej MaxLongestSidePx.
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

	target := scale(source)
	bounds := target.Bounds()

	var buffer bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buffer, target); err != nil {
		return nil, 0, 0, err
	}
	if buffer.Len() > MaxNormalizedBytes {
		return nil, 0, 0, ErrImageTooLarge
	}

	return buffer.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// scale zmniejsza obraz do dopuszczalnego boku. Obrazów mniejszych nie powiększamy -
// rozciągnięta pieczątka byłaby tylko rozmyta.
func scale(source image.Image) image.Image {
	bounds := source.Bounds()
	longest := max(bounds.Dx(), bounds.Dy())
	if longest <= MaxLongestSidePx {
		return source
	}

	ratio := float64(MaxLongestSidePx) / float64(longest)
	width := max(int(float64(bounds.Dx())*ratio), 1)
	height := max(int(float64(bounds.Dy())*ratio), 1)

	target := image.NewNRGBA(image.Rect(0, 0, width, height))
	// CatmullRom, bo krawędź okrągłej pieczątki przy prostszych filtrach wychodzi schodkowa.
	draw.CatmullRom.Scale(target, target.Bounds(), source, bounds, draw.Over, nil)
	return target
}
