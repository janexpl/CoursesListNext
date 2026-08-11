package main

import (
	"reflect"
	"testing"
)

func TestParseRecipientEmails(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{
			name:  "blank value",
			value: "  \t\n ",
			want:  nil,
		},
		{
			name:  "single recipient",
			value: "kadry@example.com",
			want:  []string{"kadry@example.com"},
		},
		{
			name:  "multiple recipients preserve order",
			value: "kadry@example.com, bhp@example.com, biuro@example.com",
			want:  []string{"kadry@example.com", "bhp@example.com", "biuro@example.com"},
		},
		{
			name:  "duplicates are case insensitive",
			value: "Kadry@Example.com, kadry@example.com, BHP@example.com",
			want:  []string{"Kadry@Example.com", "BHP@example.com"},
		},
		{
			name:  "display names are removed before deduplication",
			value: "Jan Kowalski <kontakt@example.com>, Anna Nowak <KONTAKT@example.com>",
			want:  []string{"kontakt@example.com"},
		},
		{
			name:    "invalid recipient fails entire list",
			value:   "kadry@example.com, invalid-email, bhp@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRecipientEmails(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if len(got) != 0 {
					t.Fatalf("expected no recipients on error, got %v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseRecipientEmails(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
