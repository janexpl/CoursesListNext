// Package guilloche rysuje giloszowe tło drukowane pod treścią zaświadczenia.
//
// Deseń powstaje proceduralnie, a nie z wgranego pliku: jest przez to lekki (kilkadziesiąt
// kilobajtów zamiast megabajta za tło A4 w rozdzielczości druku), ostry przy każdym
// skalowaniu i nie ma pliku, o którego kopię trzeba dbać przy wdrożeniu.
//
// Format to PNG z paletą, nie SVG - z tych samych powodów co przy kodzie QR
// (patrz internal/qrcode): wydruki powstają dwoma rendererami, a stary WebKit
// w wkhtmltopdf bywa zawodny przy SVG w atrybucie src. Obrazek wraca jako data URI,
// bo renderer nie ma gwarancji dostępu do sieci.
//
// Deseń jest celowo ten sam na każdym dokumencie. Gdyby każde zaświadczenie miało własny
// wariant, przeglądarka musiałaby pobierać osobny obrazek do każdego podglądu, a wynik
// nie dałby się trzymać w cache ani policzyć raz na proces.
package guilloche

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"
)

const (
	// Płótno odpowiada proporcjom pola nadruku na stronie (ok. 186 x 226 mm) przy
	// rozdzielczości ~150 dpi. Przy niej linia o grubości jednego piksela ma ok. 0,17 mm,
	// czyli tyle co linia giloszu na papierze wartościowym. Wyżej nie ma sensu:
	// rośnie waga pliku, a druk i tak tego nie odda.
	widthPixels  = 1100
	heightPixels = 1340

	// Biała otulina w milimetrach - margines na nieprecyzyjne podawanie papieru
	// przez drukarkę, a przy okazji naturalna ramka wzoru.
	marginMM = 8.0

	// Szerokość pola nadruku w milimetrach; służy tylko do przeliczenia marginesu
	// na piksele, żeby otulina miała stałą szerokość niezależnie od rozdzielczości.
	printWidthMM = 186.0
)

// Współczynniki wzoru. Dwie rodziny linii obrócone względem siebie, każda modulowana
// sinusem w kierunku prostopadłym - ich przecięcie daje charakterystyczny splot.
// Trzecia faza to rozeta biegunowa, która rozbija regularność siatki i sprawia,
// że wzoru nie da się odtworzyć linijką.
const (
	familyAngle1 = 18 * math.Pi / 180
	familyAngle2 = -23 * math.Pi / 180

	familyFrequency    = 88.0 // ok. 28 linii na szerokość, czyli odstęp ~6,6 mm
	familyModulation   = 2.4
	familyModFrequency = 11.0

	rosetteFrequency  = 64.0
	rosetteModulation = 3.0
	rosettePetals     = 24.0

	// Rozeta jest wygaszana wokół środka. Przy małym promieniu człon kątowy zmienia się
	// tak szybko, że zamiast linii wychodzi plama z kropek - a środek strony to właśnie
	// miejsce, w którym wypada tekst zaświadczenia.
	rosetteFadeStart = 0.18
	rosetteFadeEnd   = 0.46

	// Połowa szerokości linii wyrażona w cyklach fazy. Przy odstępie ~39 px na cykl
	// daje to linię o grubości ok. 1,2 px.
	lineHalfWidth = 0.015
)

// palette idzie od bieli do najciemniejszego odcienia. Najciemniejszy ma luminancję
// ok. 0,85, więc wobec tekstu zaświadczenia (#0f172a) kontrast zostaje powyżej 12:1 -
// deseń nie przeszkadza ani w czytaniu, ani w rozpoznawaniu tekstu przez skaner.
var palette = color.Palette{
	color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	color.RGBA{R: 0xEE, G: 0xF2, B: 0xF7, A: 0xFF},
	color.RGBA{R: 0xE0, G: 0xE8, B: 0xF2, A: 0xFF},
	color.RGBA{R: 0xCE, G: 0xD9, B: 0xE8, A: 0xFF},
}

var (
	once      sync.Once
	cachedPNG []byte
	cachedURI string
	cachedErr error
)

// PNG zwraca deseń jako bajty PNG. Wynik jest liczony raz na proces - rysowanie to
// półtora miliona pikseli, a deseń się nie zmienia.
func PNG() ([]byte, error) {
	once.Do(build)
	return cachedPNG, cachedErr
}

// DataURI zwraca ten sam deseń co PNG, gotowy do wstawienia w atrybut src.
func DataURI() (string, error) {
	once.Do(build)
	return cachedURI, cachedErr
}

// marginPixels przelicza białą otulinę z milimetrów na piksele płótna.
func marginPixels(width int) int {
	return int(math.Round(marginMM * float64(width) / printWidthMM))
}

func build() {
	img := render()

	var buffer bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buffer, img); err != nil {
		cachedErr = err
		return
	}

	cachedPNG = buffer.Bytes()
	cachedURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(cachedPNG)
}

func render() *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, widthPixels, heightPixels), palette)
	margin := marginPixels(widthPixels)

	sin1, cos1 := math.Sincos(familyAngle1)
	sin2, cos2 := math.Sincos(familyAngle2)

	for y := range heightPixels {
		// Współrzędne znormalizowane do [-1, 1], żeby współczynniki wzoru nie zależały
		// od rozdzielczości płótna.
		v := float64(2*y-heightPixels) / float64(heightPixels)
		for x := range widthPixels {
			if x < margin || y < margin || x >= widthPixels-margin || y >= heightPixels-margin {
				continue
			}
			u := float64(2*x-widthPixels) / float64(widthPixels)

			along1 := u*cos1 + v*sin1
			across1 := v*cos1 - u*sin1
			phase1 := familyFrequency*along1 + familyModulation*math.Sin(familyModFrequency*across1)

			along2 := u*cos2 + v*sin2
			across2 := v*cos2 - u*sin2
			phase2 := familyFrequency*along2 + familyModulation*math.Sin(familyModFrequency*across2)

			radius := math.Hypot(u, v)
			angle := math.Atan2(v, u)
			// Człon kątowy przemnożony przez promień: bez tego płatki rozety zbiegają się
			// w środku szybciej, niż da się je narysować pikselami.
			phase3 := rosetteFrequency*radius + rosetteModulation*radius*math.Sin(rosettePetals*angle)
			rosette := lineIntensity(phase3) * radialFade(radius)

			ink := math.Max(lineIntensity(phase1), math.Max(lineIntensity(phase2), rosette))
			if ink <= 0 {
				continue
			}
			img.SetColorIndex(x, y, level(ink))
		}
	}

	return img
}

// lineIntensity zwraca nasycenie atramentu w punkcie o danej fazie: linia biegnie tam,
// gdzie faza mija wielokrotność 2π, a jej brzegi są wygaszane, żeby nie postrzępić wzoru.
func lineIntensity(phase float64) float64 {
	cycles := phase / (2 * math.Pi)
	distance := math.Abs(cycles - math.Round(cycles))
	if distance >= lineHalfWidth {
		return 0
	}
	return smoothstep(1 - distance/lineHalfWidth)
}

func smoothstep(t float64) float64 {
	return t * t * (3 - 2*t)
}

// radialFade wygasza rozetę w stronę środka strony.
func radialFade(radius float64) float64 {
	if radius <= rosetteFadeStart {
		return 0
	}
	if radius >= rosetteFadeEnd {
		return 1
	}
	return smoothstep((radius - rosetteFadeStart) / (rosetteFadeEnd - rosetteFadeStart))
}

// level odwzorowuje nasycenie na indeks palety, pomijając biel (indeks 0).
func level(ink float64) uint8 {
	index := 1 + int(ink*float64(len(palette)-1))
	if index >= len(palette) {
		index = len(palette) - 1
	}
	return uint8(index)
}
