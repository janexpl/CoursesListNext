package validation

import (
	"reflect"
	"testing"
)

func TestParseEmailList(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{
			name:  "single address",
			value: "kadry@example.com",
			want:  []string{"kadry@example.com"},
		},
		{
			name:  "multiple addresses preserve order",
			value: "kadry@example.com, bhp@example.com, biuro@example.com",
			want:  []string{"kadry@example.com", "bhp@example.com", "biuro@example.com"},
		},
		{
			name:  "surrounding whitespace",
			value: "  kadry@example.com, bhp@example.com  ",
			want:  []string{"kadry@example.com", "bhp@example.com"},
		},
		{
			name:  "blank value",
			value: "  \t\n ",
			want:  nil,
		},
		{
			name:    "invalid address in the middle",
			value:   "kadry@example.com, invalid-email, bhp@example.com",
			wantErr: true,
		},
		{
			name:  "case insensitive duplicates preserve first address",
			value: "Kadry@Example.com, kadry@example.com, BHP@example.com",
			want:  []string{"Kadry@Example.com", "BHP@example.com"},
		},
		{
			name:  "display name is removed",
			value: "Jan Kowalski <jan@example.com>",
			want:  []string{"jan@example.com"},
		},
		{
			name:  "same address with different display names is deduplicated",
			value: "Jan Kowalski <kontakt@example.com>, Anna Nowak <KONTAKT@example.com>",
			want:  []string{"kontakt@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEmailList(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if len(got) != 0 {
					t.Fatalf("expected no addresses on error, got %v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseEmailList(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
