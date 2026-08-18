package auth

import (
	"crypto/rand"
	"encoding/base64"
)

// APIKeyTokenPrefix odróżnia klucz API od tokenu sesji na pierwszy rzut oka -
// także w logach i skanerach sekretów.
const APIKeyTokenPrefix = "clk_"

// apiKeyPrefixLength to długość jawnego fragmentu klucza zapisywanego w bazie.
// Służy tylko do rozpoznania klucza na liście w panelu; pełny klucz ma 256 bitów
// entropii, więc ujawnienie 8 znaków nie zbliża nikogo do jego odgadnięcia.
const apiKeyPrefixLength = len(APIKeyTokenPrefix) + 8

// NewAPIKeyToken generuje klucz API i wartości, które trafiają do bazy.
// Surowy klucz nie jest nigdzie zapisywany - wraca do wołającego, który pokazuje
// go użytkownikowi jeden raz.
func NewAPIKeyToken() (raw string, prefix string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	raw = APIKeyTokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return raw, raw[:apiKeyPrefixLength], HashToken(raw), nil
}

// HashToken zwraca wartość zapisywaną w bazie zamiast surowego poświadczenia.
func HashToken(raw string) string {
	return hashToken(raw)
}
