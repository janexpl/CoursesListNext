package validation

import (
	"errors"
	"math"
	"net/mail"
	"strings"
)

var (
	ErrInvalidLength   = errors.New("nip must contain exactly 10 digits")
	ErrInvalidChecksum = errors.New("invalid nip checksum")
	ErrInvalidFormat   = errors.New("nip contains invalid characters")
)

func CheckEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

type Unsigned interface {
	~uint | ~uint32 | ~uint64
}

func UnsignedToInt64Clamped[T Unsigned](u T) int64 {
	if uint64(u) > uint64(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(u)
}

type Signed interface {
	~int | ~int32 | ~int64
}

func SignedToInt64Clamped[T Signed](u T) int64 {
	if int64(u) > int64(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(u)
}

func Int64ToInt32(number int64) int32 {
	if number > math.MaxInt32 {
		return math.MaxInt32
	}
	if number < math.MinInt32 {
		return math.MinInt32
	}
	return int32(number)
}

func ValidateNIP(nip string) error {
	digits := NormalizeNIP(nip)

	if len(digits) != 10 {
		return ErrInvalidLength
	}

	weights := [9]int{6, 5, 7, 2, 3, 4, 5, 6, 7}

	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(digits[i]-'0') * weights[i]
	}

	checksum := sum % 11
	if checksum == 10 {
		return ErrInvalidChecksum
	}

	if checksum != int(digits[9]-'0') {
		return ErrInvalidChecksum
	}

	return nil
}

func NormalizeNIP(value string) string {
	replacer := strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "")
	return replacer.Replace(strings.TrimSpace(value))
}
