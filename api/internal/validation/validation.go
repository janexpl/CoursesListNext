package validation

import (
	"math"
	"net/mail"
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
