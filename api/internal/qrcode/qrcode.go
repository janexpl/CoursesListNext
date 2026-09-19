// Package qrcode buduje kody QR osadzane bezpośrednio w dokumencie.
//
// Obrazek wraca jako data URI, bo renderer PDF dostaje HTML plikiem tymczasowym
// (chromium) albo przez standardowe wejście (wkhtmltopdf) i nie ma gwarancji dostępu
// do sieci - obrazek pobierany z adresu mógłby się nie wczytać albo wydłużyć
// renderowanie ponad limit czasu.
//
// Format to PNG, nie SVG: wydruki powstają dwoma rendererami, a stary WebKit
// w wkhtmltopdf bywa zawodny przy SVG w atrybucie src. PNG w tej rozdzielczości
// drukuje się ostro (przy 24 mm to ponad 500 dpi) i zachowuje się tak samo
// w obu rendererach oraz w podglądzie w przeglądarce.
package qrcode

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"

	"rsc.io/qr"
)

const (
	// recoveryLevel Q (25%) zamiast domyślnego M: zaświadczenie bywa pieczętowane,
	// podpisywane, składane i kopiowane, a kod ma to przetrwać. Kosztuje jedną
	// wersję kodu w górę, czyli nieco gęstsze moduły.
	recoveryLevel = qr.Q

	// quietZoneModules to biała otulina wymagana przez standard - bez niej czytniki
	// gubią kod na tle dokumentu.
	quietZoneModules = 4

	// minimumSidePixels wyznacza rozdzielczość obrazka. Przy druku 24 mm daje ponad
	// 500 dpi, więc kod pozostaje ostry także po przeskalowaniu przez drukarkę.
	minimumSidePixels = 500

	// verificationCodePlaceholder to znacznik podmieniany w adresie weryfikacji.
	verificationCodePlaceholder = "{code}"
)

// ErrEmptyContent - nie ma czego zakodować.
var ErrEmptyContent = errors.New("qrcode: empty content")

// VerificationURL składa adres publicznej weryfikacji z wzorca konfiguracji i kodu
// weryfikacyjnego zaświadczenia. Pusty wzorzec albo pusty kod dają pusty adres,
// co dla wywołującego oznacza "nie drukuj kodu QR".
func VerificationURL(template, code string) string {
	template = strings.TrimSpace(template)
	code = strings.TrimSpace(code)
	if template == "" || code == "" {
		return ""
	}

	return strings.ReplaceAll(template, verificationCodePlaceholder, code)
}

// PNGDataURI zwraca kod QR o podanej treści jako data URI z obrazem PNG.
func PNGDataURI(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", ErrEmptyContent
	}

	code, err := qr.Encode(content, recoveryLevel)
	if err != nil {
		return "", err
	}

	img := renderCode(code)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return "", err
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

// renderCode maluje matrycę kodu wraz z otuliną. Skala jest całkowita, żeby moduły
// miały identyczną szerokość - przy skali ułamkowej sąsiednie moduły różniłyby się
// o piksel i czytnik miałby trudniej.
func renderCode(code *qr.Code) image.Image {
	modules := code.Size + 2*quietZoneModules
	scale := (minimumSidePixels + modules - 1) / modules
	side := modules * scale

	img := image.NewGray(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	black := color.Gray{Y: 0x00}
	for y := range code.Size {
		for x := range code.Size {
			if !code.Black(x, y) {
				continue
			}
			for dy := range scale {
				for dx := range scale {
					img.SetGray(
						(x+quietZoneModules)*scale+dx,
						(y+quietZoneModules)*scale+dy,
						black,
					)
				}
			}
		}
	}

	return img
}
