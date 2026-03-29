// Package pgutil - utils for slqc pgtype
package pgutil

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

func NullableDate(value pgtype.Date) *string {
	if !value.Valid {
		return nil
	}

	formatted := value.Time.Format(response.DateFormat)
	return &formatted
}

func NullableTimestampz(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}

	formatted := value.Time.Format(response.TimestampzFormat)
	return &formatted
}

func NullableString(value any) *string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		return &v
	case []byte:
		if len(v) == 0 {
			return nil
		}
		s := string(v)
		return &s
	case pgtype.Text:
		if !v.Valid || v.String == "" {
			return nil
		}
		return &v.String
	default:
		return nil
	}
}

func OptionalText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return pgtype.Text{}
	}

	return pgtype.Text{
		String: trimmed,
		Valid:  true,
	}
}

func OptionalInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}

	return pgtype.Int8{
		Int64: *value,
		Valid: true,
	}
}

func NullableInt64(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}
