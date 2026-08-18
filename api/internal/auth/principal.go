package auth

import (
	"context"

	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

// Method mówi, czym uwierzytelniono żądanie.
type Method string

const (
	MethodSession Method = "session"
	MethodAPIKey  Method = "api_key"
)

// Principal to tożsamość stojąca za żądaniem: użytkownik plus informacja, skąd
// się wziął. Handlery nadal czytają wyłącznie użytkownika przez UserFromContext,
// więc dodanie klucza API niczego w nich nie zmienia.
type Principal struct {
	User       dbsqlc.User
	Method     Method
	APIKeyID   int64
	APIKeyName string
	// Scopes jest puste dla sesji - uprawnienia zalogowanego użytkownika
	// wynikają wyłącznie z jego roli.
	Scopes []string
}

// ContextWithPrincipal zapisuje tożsamość w kontekście. Użytkownik trafia
// dodatkowo pod własny klucz, żeby UserFromContext działał niezależnie od tego,
// czy wołający wie o istnieniu Principal.
func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	ctx = context.WithValue(ctx, principalContextKey, principal)
	return context.WithValue(ctx, userContextKey, principal.User)
}

// PrincipalFromContext zwraca tożsamość zapisaną przez Authenticate.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}
