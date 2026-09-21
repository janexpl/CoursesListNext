// Package certassets zarządza nadrukami zaświadczeń: pieczątkami i podpisem, które
// administrator wgrywa raz dla całego rejestru.
//
// Nadruki trafiają wyłącznie na zaświadczenia wystawiane przez platformę e-learningową -
// taki dokument idzie do kursanta elektronicznie i nikt nie przystawia na nim pieczątki
// ręcznie. Decyzję, czy dokument jest platformowy, podejmuje pakiet certificates
// (webhooks.IsPlatformCertificate); ten pakiet odpowiada tylko za pliki.
package certassets

// Rodzaje nadruków. Nazwy są celowo znaczeniowe, a nie numerowane: są zarazem nazwami
// znaczników szablonu ({{ pieczatka_okragla }} i pozostałe), więc autor szablonu widzi,
// którą pieczątkę wstawia, bez zaglądania do dokumentacji.
//
// Każdy nadruk jest opcjonalny - dopóki administrator nie wgra pliku, znacznik znika
// z wydruku bez śladu.
const (
	KindStampRound    = "pieczatka_okragla"
	KindStampCompany  = "pieczatka_firmowa"
	KindStampPersonal = "pieczatka_imienna"
	KindSignature     = "podpis"
)

// AllKinds zwraca rodzaje w kolejności, w jakiej pojawiają się na pasku u dołu wydruku.
func AllKinds() []string {
	return []string{KindStampRound, KindStampCompany, KindStampPersonal, KindSignature}
}

// IsValidKind mówi, czy rodzaj jest obsługiwany. Ta sama lista stoi w ograniczeniu
// CHECK tabeli certificate_print_assets.
func IsValidKind(kind string) bool {
	switch kind {
	case KindStampRound, KindStampCompany, KindStampPersonal, KindSignature:
		return true
	default:
		return false
	}
}

// DefaultWidthMM to szerokość nadruku proponowana przy pierwszym wgraniu pliku.
// Pieczątka okrągła jest kwadratowa, firmowa i imienna zwykle prostokątne i szersze,
// podpis najszerszy i najniższy.
func DefaultWidthMM(kind string) int {
	switch kind {
	case KindSignature:
		return 45
	case KindStampCompany:
		return 40
	case KindStampPersonal:
		return 35
	default:
		return 30
	}
}
