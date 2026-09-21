// Package certassets zarządza nadrukami zaświadczeń: pieczątkami i podpisem, które
// administrator wgrywa raz dla całego rejestru.
//
// Nadruki trafiają wyłącznie na zaświadczenia wystawiane przez platformę e-learningową -
// taki dokument idzie do kursanta elektronicznie i nikt nie przystawia na nim pieczątki
// ręcznie. Decyzję, czy dokument jest platformowy, podejmuje pakiet certificates
// (webhooks.IsPlatformCertificate); ten pakiet odpowiada tylko za pliki.
package certassets

// Rodzaje nadruków. Wartości są celowo identyczne ze znacznikami szablonu
// ({{ pieczatka_1 }} i pozostałe), żeby nie trzeba było utrzymywać tabelki mapującej
// nazwę rodzaju na nazwę znacznika - literówka w jednym miejscu rozjechałaby drugie.
const (
	KindStamp1    = "pieczatka_1"
	KindStamp2    = "pieczatka_2"
	KindSignature = "podpis"
)

// AllKinds zwraca rodzaje w kolejności, w jakiej pojawiają się na wydruku.
func AllKinds() []string {
	return []string{KindStamp1, KindStamp2, KindSignature}
}

// IsValidKind mówi, czy rodzaj jest obsługiwany. Ta sama lista stoi w ograniczeniu
// CHECK tabeli certificate_print_assets.
func IsValidKind(kind string) bool {
	switch kind {
	case KindStamp1, KindStamp2, KindSignature:
		return true
	default:
		return false
	}
}

// DefaultWidthMM to szerokość nadruku proponowana przy pierwszym wgraniu pliku.
// Pieczątki są zwykle kwadratowe albo okrągłe, podpis jest szeroki i niski.
func DefaultWidthMM(kind string) int {
	if kind == KindSignature {
		return 50
	}
	return 35
}
