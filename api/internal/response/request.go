package response

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

const DefaultLimit = 50

var ErrInvalidPathValue = errors.New("invalid path value")

func ParsePositiveInt64PathValue(r *http.Request, key string) (int64, error) {
	value := r.PathValue(key)
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, ErrInvalidPathValue
	}

	return parsed, nil
}

func ParseListParams(r *http.Request) (pgtype.Text, int32, error) {
	limitInt := DefaultLimit
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	pgSearch := pgtype.Text{}
	if search != "" {
		pgSearch = pgtype.Text{
			String: search,
			Valid:  true,
		}
	}
	limit := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limit != "" {
		parsedLimit, err := strconv.Atoi(limit)
		if err != nil {
			return pgtype.Text{}, 0, errors.New("failed to convert limit value")
		}
		if parsedLimit < 1 || parsedLimit > 100 {
			return pgtype.Text{}, 0, errors.New("incorrect limit value")
		}
		limitInt = parsedLimit
	}
	return pgSearch, int32(limitInt), nil

}
