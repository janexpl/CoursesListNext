package auth

import (
	"slices"
	"strings"
)

// Zakresy zawężają uprawnienia klucza API do konkretnych zasobów. Nie zastępują
// kontroli roli: żądanie uwierzytelnione kluczem przechodzi i przez RequireScope,
// i przez RequireAdmin, więc klucz konta bez roli admina nie dostanie dostępu do
// tras administracyjnych nawet z zakresem users:write.
//
// Konwencja: "<zasób>:<akcja>", gdzie akcja to read albo write.
// write implikuje read na tym samym zasobie (patrz HasScope).
const (
	ScopeStudentsRead     = "students:read"
	ScopeStudentsWrite    = "students:write"
	ScopeCompaniesRead    = "companies:read"
	ScopeCompaniesWrite   = "companies:write"
	ScopeCoursesRead      = "courses:read"
	ScopeCoursesWrite     = "courses:write"
	ScopeCertificatesRead = "certificates:read"
	//nolint:gosec // nazwa zakresu uprawnień, nie poświadczenie
	ScopeCertificatesWrite = "certificates:write"
	ScopeJournalsRead      = "journals:read"
	ScopeJournalsWrite     = "journals:write"
	ScopeRegistriesRead    = "registries:read"
	ScopeDashboardRead     = "dashboard:read"
	ScopeUsersRead         = "users:read"
	ScopeUsersWrite        = "users:write"
	ScopeAuditLogRead      = "audit-log:read"
)

// assignableScopes to zamknięta lista zakresów, które można przypisać kluczowi.
// Wszystko spoza niej jest odrzucane przy tworzeniu klucza, żeby literówka nie
// tworzyła cicho klucza, który nigdzie nie zadziała.
var assignableScopes = map[string]struct{}{
	ScopeStudentsRead:      {},
	ScopeStudentsWrite:     {},
	ScopeCompaniesRead:     {},
	ScopeCompaniesWrite:    {},
	ScopeCoursesRead:       {},
	ScopeCoursesWrite:      {},
	ScopeCertificatesRead:  {},
	ScopeCertificatesWrite: {},
	ScopeJournalsRead:      {},
	ScopeJournalsWrite:     {},
	ScopeRegistriesRead:    {},
	ScopeDashboardRead:     {},
	ScopeUsersRead:         {},
	ScopeUsersWrite:        {},
	ScopeAuditLogRead:      {},
}

// AssignableScopes zwraca posortowaną listę zakresów do wyboru w panelu admina.
func AssignableScopes() []string {
	scopes := make([]string, 0, len(assignableScopes))
	for scope := range assignableScopes {
		scopes = append(scopes, scope)
	}
	slices.Sort(scopes)
	return scopes
}

// IsAssignableScope mówi, czy zakres istnieje i można go przypisać kluczowi.
func IsAssignableScope(scope string) bool {
	_, ok := assignableScopes[scope]
	return ok
}

// HasScope sprawdza, czy przyznane zakresy pokrywają wymagany.
// Zakres zapisu implikuje odczyt tego samego zasobu, więc trasy GET wymagają
// tylko "<zasób>:read", a klucz z "<zasób>:write" nadal je przejdzie.
func HasScope(granted []string, required string) bool {
	resource, action, ok := strings.Cut(required, ":")
	writeEquivalent := ""
	if ok && action == "read" {
		writeEquivalent = resource + ":write"
	}

	for _, scope := range granted {
		if scope == required {
			return true
		}
		if writeEquivalent != "" && scope == writeEquivalent {
			return true
		}
	}
	return false
}
