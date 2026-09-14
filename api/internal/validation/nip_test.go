package validation

import (
	"errors"
	"testing"
)

func TestValidateNIP(t *testing.T) {
	tests := []struct {
		name string
		nip  string
		want error
	}{
		{name: "poprawny", nip: "1234563218", want: nil},
		{name: "poprawny z myślnikami", nip: "123-456-32-18", want: nil},
		{name: "zła suma kontrolna", nip: "1234567890", want: ErrInvalidChecksum},
		{name: "za krótki", nip: "123456789", want: ErrInvalidLength},
		{name: "za długi", nip: "12345632181", want: ErrInvalidLength},
		// Wcześniej litery przechodziły kontrolę długości i trafiały do sumy
		// kontrolnej jako przypadkowe wartości.
		{name: "litery", nip: "12345abcde", want: ErrInvalidFormat},
		{name: "litera na końcu", nip: "123456321X", want: ErrInvalidFormat},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNIP(tc.nip)
			if tc.want == nil && err != nil {
				t.Fatalf("expected %q to be valid, got %v", tc.nip, err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("expected %v for %q, got %v", tc.want, tc.nip, err)
			}
		})
	}
}
