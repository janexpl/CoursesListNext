// Package guilloche dostarcza giloszowe tło drukowane pod treścią zaświadczenia.
//
// Wzór pochodzi z blankietu używanego przez organizatora (docs/api, skan 200 dpi)
// i został przygotowany do druku: wyprostowany, przycięty do arkusza, pozbawiony flagi
// UE i przemalowany na jednolitą zieleń, a następnie zapisany jako PNG z ośmiokolorową
// paletą. Paleta zamiast pełnego RGB, bo rysunek jest jednobarwny - to różnica między
// 1,2 MB a 300 kB na stronę, a całość i tak wędruje w każdym PDF-ie.
//
// Wersja proceduralna, którą ten pakiet miał wcześniej, rysowała wzór z funkcji
// trygonometrycznych. Wyglądała poprawnie, ale nie miała nic wspólnego z blankietem
// organizatora - a dokument ma wyglądać jak ten, który ludzie znają z papieru.
//
// Osobne wzory na przód i odwrót: przód ma kaligraficzny monogram na środku, odwrót
// samą ramkę i siatkę, dokładnie jak w blankiecie.
package guilloche

import (
	_ "embed"
	"encoding/base64"
	"sync"
)

// Wzory są wkompilowane w binarkę, a nie trzymane w bazie ani na dysku: są takie same
// dla każdego dokumentu i nie zmieniają się bez wdrożenia, więc nie ma czego konfigurować.
//
//go:embed pattern/front.png
var frontPNG []byte

//go:embed pattern/back.png
var backPNG []byte

var (
	once     sync.Once
	frontURI string
	backURI  string
)

// FrontPNG zwraca wzór pierwszej strony (ramka, siatka i monogram).
func FrontPNG() []byte { return frontPNG }

// BackPNG zwraca wzór odwrotu (ramka i siatka, bez monogramu).
func BackPNG() []byte { return backPNG }

// FrontDataURI zwraca wzór pierwszej strony gotowy do wstawienia w atrybut src.
// Kodowanie base64 liczone jest raz na proces - wzór się nie zmienia, a kodowanie
// 300 kB przy każdym wydruku byłoby czystą stratą.
func FrontDataURI() string {
	once.Do(encode)
	return frontURI
}

// BackDataURI zwraca wzór odwrotu jako data URI.
func BackDataURI() string {
	once.Do(encode)
	return backURI
}

func encode() {
	frontURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(frontPNG)
	backURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(backPNG)
}
