package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const dateHint = "Data w formacie RRRR-MM-DD."

var (
	readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true}
	// Zapis, który tylko dodaje dane (nowy kursant, zaświadczenie, uczestnik).
	additive = &mcp.ToolAnnotations{DestructiveHint: new(false)}
	// Zapis, który nadpisuje istniejące dane.
	overwrite = &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true}
)

// addTool rejestruje narzędzie, które zwraca agentowi odpowiedź API jako JSON.
// Błąd API (np. 400 z komunikatem walidacji) trafia do agenta jako błąd narzędzia,
// więc agent może poprawić dane i spróbować ponownie.
func addTool[In any](s *mcp.Server, name, description string, annotations *mcp.ToolAnnotations, call func(context.Context, In) ([]byte, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, Annotations: annotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			raw, err := call(ctx, in)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
		})
}

type searchInput struct {
	Search string `json:"search,omitempty" jsonschema:"Fraza do wyszukania; puste zwraca najnowsze wpisy."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maksymalna liczba wyników, 1-100, domyślnie 50."`
}

func (in searchInput) query() url.Values {
	q := url.Values{}
	setText(q, "search", in.Search)
	setInt(q, "limit", int64(in.Limit))
	return q
}

type idInput struct {
	ID int64 `json:"id" jsonschema:"Identyfikator rekordu."`
}

type studentFields struct {
	FirstName     string  `json:"firstName" jsonschema:"Imię."`
	LastName      string  `json:"lastName" jsonschema:"Nazwisko."`
	SecondName    *string `json:"secondName,omitempty" jsonschema:"Drugie imię."`
	BirthDate     string  `json:"birthDate" jsonschema:"Data urodzenia, RRRR-MM-DD."`
	BirthPlace    string  `json:"birthPlace" jsonschema:"Miejsce urodzenia."`
	Pesel         *string `json:"pesel,omitempty" jsonschema:"PESEL albo numer paszportu cudzoziemca; nie jest sprawdzany."`
	AddressStreet *string `json:"addressStreet,omitempty"`
	AddressCity   *string `json:"addressCity,omitempty"`
	AddressZip    *string `json:"addressZip,omitempty"`
	Telephone     *string `json:"telephone,omitempty"`
	CompanyID     *int64  `json:"companyId,omitempty" jsonschema:"Identyfikator firmy kursanta."`
}

type updateStudentInput struct {
	ID            int64   `json:"id" jsonschema:"Identyfikator kursanta."`
	FirstName     *string `json:"firstName,omitempty"`
	LastName      *string `json:"lastName,omitempty"`
	SecondName    *string `json:"secondName,omitempty"`
	BirthDate     *string `json:"birthDate,omitempty" jsonschema:"RRRR-MM-DD."`
	BirthPlace    *string `json:"birthPlace,omitempty"`
	Pesel         *string `json:"pesel,omitempty"`
	AddressStreet *string `json:"addressStreet,omitempty"`
	AddressCity   *string `json:"addressCity,omitempty"`
	AddressZip    *string `json:"addressZip,omitempty"`
	Telephone     *string `json:"telephone,omitempty"`
	CompanyID     *int64  `json:"companyId,omitempty" jsonschema:"Nowa firma kursanta."`
}

type companyFields struct {
	Name          string  `json:"name" jsonschema:"Nazwa firmy."`
	Street        string  `json:"street"`
	City          string  `json:"city"`
	Zipcode       string  `json:"zipcode" jsonschema:"Kod pocztowy, np. 00-001."`
	NIP           string  `json:"nip" jsonschema:"NIP, 10 cyfr."`
	Email         *string `json:"email,omitempty"`
	ContactPerson *string `json:"contactPerson,omitempty"`
	Telephone     *string `json:"telephone,omitempty"`
	Note          *string `json:"note,omitempty"`
}

type nipInput struct {
	NIP string `json:"nip" jsonschema:"NIP firmy, 10 cyfr."`
}

type certificateSearchInput struct {
	searchInput
	DateFrom string `json:"dateFrom,omitempty" jsonschema:"Data wystawienia od, RRRR-MM-DD."`
	DateTo   string `json:"dateTo,omitempty" jsonschema:"Data wystawienia do, RRRR-MM-DD."`
}

type issueCertificateInput struct {
	StudentID       int64  `json:"studentId"`
	CourseID        int64  `json:"courseId"`
	CertificateDate string `json:"certificateDate" jsonschema:"Data wystawienia, RRRR-MM-DD."`
	CourseDateStart string `json:"courseDateStart" jsonschema:"Pierwszy dzień szkolenia, RRRR-MM-DD."`
	CourseDateEnd   string `json:"courseDateEnd,omitempty" jsonschema:"Ostatni dzień szkolenia, RRRR-MM-DD; puste dla szkolenia jednodniowego."`
	LanguageCode    string `json:"languageCode,omitempty" jsonschema:"Wersja językowa, np. en; domyślnie polska."`
}

type renewCertificateInput struct {
	ID              int64  `json:"id" jsonschema:"Identyfikator przedłużanego zaświadczenia."`
	CertificateDate string `json:"certificateDate" jsonschema:"Data wystawienia nowego zaświadczenia, RRRR-MM-DD."`
	CourseDateStart string `json:"courseDateStart" jsonschema:"RRRR-MM-DD."`
	CourseDateEnd   string `json:"courseDateEnd,omitempty" jsonschema:"RRRR-MM-DD."`
	LanguageCode    string `json:"languageCode,omitempty"`
}

type journalSearchInput struct {
	searchInput
	CourseID  int64  `json:"courseId,omitempty"`
	CompanyID int64  `json:"companyId,omitempty"`
	Status    string `json:"status,omitempty" jsonschema:"draft (roboczy) albo closed (zamknięty)."`
	DateFrom  string `json:"dateFrom,omitempty" jsonschema:"RRRR-MM-DD."`
	DateTo    string `json:"dateTo,omitempty" jsonschema:"RRRR-MM-DD."`
}

type createJournalInput struct {
	CourseID         int64   `json:"courseId"`
	CompanyID        *int64  `json:"companyId,omitempty" jsonschema:"Firma, dla której jest szkolenie zamknięte."`
	Title            string  `json:"title"`
	OrganizerName    string  `json:"organizerName" jsonschema:"Organizator; zostaje też prowadzącym wygenerowanych sesji."`
	OrganizerAddress *string `json:"organizerAddress,omitempty"`
	Location         string  `json:"location" jsonschema:"Miejsce szkolenia."`
	FormOfTraining   string  `json:"formOfTraining" jsonschema:"Forma szkolenia, np. stacjonarna."`
	LegalBasis       string  `json:"legalBasis,omitempty" jsonschema:"Podstawa prawna; puste = podstawa przypisana do kursu."`
	DateStart        string  `json:"dateStart" jsonschema:"RRRR-MM-DD."`
	DateEnd          string  `json:"dateEnd" jsonschema:"RRRR-MM-DD; program musi się zmieścić po 8 godzin lekcyjnych dziennie."`
	Notes            *string `json:"notes,omitempty"`
}

type journalAttendeeInput struct {
	JournalID int64 `json:"journalId"`
	StudentID int64 `json:"studentId"`
}

type attendanceInput struct {
	JournalID         int64 `json:"journalId"`
	JournalSessionID  int64 `json:"journalSessionId" jsonschema:"Identyfikator sesji z get_journal."`
	JournalAttendeeID int64 `json:"journalAttendeeId" jsonschema:"Identyfikator uczestnika dziennika z get_journal (nie kursanta)."`
	Present           bool  `json:"present"`
}

type attendeeCertificateInput struct {
	JournalID  int64 `json:"journalId"`
	AttendeeID int64 `json:"attendeeId" jsonschema:"Identyfikator uczestnika dziennika z get_journal."`
}

func newServer(api *apiClient) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "courseslist", Version: "1.0.0"}, &mcp.ServerOptions{
		Instructions: "System ewidencji szkoleń: kursanci, firmy, kursy, zaświadczenia i dzienniki zajęć. " +
			"Dane są po polsku. Zanim coś utworzysz, wyszukaj, czy już nie istnieje. " + dateHint,
	})

	// Kursanci
	addTool(s, "search_students", "Wyszukuje kursantów po imieniu, nazwisku lub PESEL.", readOnly,
		func(ctx context.Context, in searchInput) ([]byte, error) {
			return api.get(ctx, "/students", in.query())
		})
	addTool(s, "get_student", "Pełne dane kursanta.", readOnly,
		func(ctx context.Context, in idInput) ([]byte, error) {
			return api.get(ctx, fmt.Sprintf("/students/%d", in.ID), nil)
		})
	addTool(s, "list_student_certificates", "Zaświadczenia wystawione kursantowi.", readOnly,
		func(ctx context.Context, in idInput) ([]byte, error) {
			return api.get(ctx, fmt.Sprintf("/students/%d/certificates", in.ID), nil)
		})
	addTool(s, "create_student", "Dodaje kursanta.", additive,
		func(ctx context.Context, in studentFields) ([]byte, error) {
			return api.send(ctx, http.MethodPost, "/students", in)
		})
	addTool(s, "update_student", "Zmienia wybrane dane kursanta; pominięte pola zostają bez zmian.", overwrite,
		func(ctx context.Context, in updateStudentInput) ([]byte, error) {
			return updateStudent(ctx, api, in)
		})

	// Firmy
	addTool(s, "search_companies", "Wyszukuje firmy po nazwie lub NIP.", readOnly,
		func(ctx context.Context, in searchInput) ([]byte, error) {
			return api.get(ctx, "/companies", in.query())
		})
	addTool(s, "get_company", "Pełne dane firmy.", readOnly,
		func(ctx context.Context, in idInput) ([]byte, error) {
			return api.get(ctx, fmt.Sprintf("/companies/%d", in.ID), nil)
		})
	addTool(s, "lookup_company_by_nip", "Pobiera dane firmy z rejestru GUS po NIP (do wypełnienia create_company).", readOnly,
		func(ctx context.Context, in nipInput) ([]byte, error) {
			return api.get(ctx, "/companies/lookup-by-nip", url.Values{"nip": {in.NIP}})
		})
	addTool(s, "create_company", "Dodaje firmę.", additive,
		func(ctx context.Context, in companyFields) ([]byte, error) {
			return api.send(ctx, http.MethodPost, "/companies", in)
		})

	// Kursy
	addTool(s, "list_courses", "Wyszukuje kursy po nazwie lub symbolu.", readOnly,
		func(ctx context.Context, in searchInput) ([]byte, error) {
			return api.get(ctx, "/courses", in.query())
		})
	addTool(s, "get_course", "Kurs z programem (godziny lekcyjne) i podstawą prawną.", readOnly,
		func(ctx context.Context, in idInput) ([]byte, error) {
			return api.get(ctx, fmt.Sprintf("/courses/%d", in.ID), nil)
		})

	// Zaświadczenia
	addTool(s, "search_certificates", "Wyszukuje zaświadczenia po kursancie, kursie lub numerze.", readOnly,
		func(ctx context.Context, in certificateSearchInput) ([]byte, error) {
			q := in.query()
			setText(q, "dateFrom", in.DateFrom)
			setText(q, "dateTo", in.DateTo)
			return api.get(ctx, "/certificates", q)
		})
	addTool(s, "get_certificate", "Pełne dane zaświadczenia.", readOnly,
		func(ctx context.Context, in idInput) ([]byte, error) {
			return api.get(ctx, fmt.Sprintf("/certificates/%d", in.ID), nil)
		})
	addTool(s, "get_dashboard", "Statystyki i zaświadczenia, którym kończy się ważność.", readOnly,
		func(ctx context.Context, _ struct{}) ([]byte, error) {
			return api.get(ctx, "/dashboard", nil)
		})
	addTool(s, "issue_certificate", "Wystawia zaświadczenie; numer w rejestrze nadaje serwer.", additive,
		func(ctx context.Context, in issueCertificateInput) ([]byte, error) {
			return api.sendIdempotent(ctx, http.MethodPost, "/certificates", in)
		})
	addTool(s, "renew_certificate", "Przedłuża zaświadczenie: wystawia nowe z kolejnym numerem, stare znika z wygasających.", additive,
		func(ctx context.Context, in renewCertificateInput) ([]byte, error) {
			return api.sendIdempotent(ctx, http.MethodPost, fmt.Sprintf("/certificates/%d/renew", in.ID), renewBody{
				CertificateDate: in.CertificateDate,
				CourseDateStart: in.CourseDateStart,
				CourseDateEnd:   in.CourseDateEnd,
				LanguageCode:    in.LanguageCode,
			})
		})

	// Dzienniki
	addTool(s, "list_journals", "Wyszukuje dzienniki zajęć.", readOnly,
		func(ctx context.Context, in journalSearchInput) ([]byte, error) {
			q := in.query()
			setInt(q, "courseId", in.CourseID)
			setInt(q, "companyId", in.CompanyID)
			setText(q, "status", in.Status)
			setText(q, "dateFrom", in.DateFrom)
			setText(q, "dateTo", in.DateTo)
			return api.get(ctx, "/journals", q)
		})
	addTool(s, "get_journal", "Dziennik razem z sesjami, uczestnikami i obecnościami.", readOnly,
		func(ctx context.Context, in idInput) ([]byte, error) {
			return getJournal(ctx, api, in.ID)
		})
	addTool(s, "create_journal", "Zakłada dziennik i generuje sesje z programu kursu.", additive,
		func(ctx context.Context, in createJournalInput) ([]byte, error) {
			return api.send(ctx, http.MethodPost, "/journals", in)
		})
	addTool(s, "add_journal_attendee", "Dopisuje kursanta do dziennika.", additive,
		func(ctx context.Context, in journalAttendeeInput) ([]byte, error) {
			return api.send(ctx, http.MethodPost, fmt.Sprintf("/journals/%d/attendees", in.JournalID),
				map[string]int64{"studentId": in.StudentID})
		})
	addTool(s, "set_attendance", "Zaznacza obecność lub nieobecność uczestnika na sesji.", overwrite,
		func(ctx context.Context, in attendanceInput) ([]byte, error) {
			return api.send(ctx, http.MethodPatch, fmt.Sprintf("/journals/%d/attendance", in.JournalID), map[string]any{
				"journalSessionId":  in.JournalSessionID,
				"journalAttendeeId": in.JournalAttendeeID,
				"present":           in.Present,
			})
		})
	addTool(s, "generate_attendee_certificate", "Wystawia zaświadczenie uczestnikowi dziennika na podstawie danych dziennika.", additive,
		func(ctx context.Context, in attendeeCertificateInput) ([]byte, error) {
			return api.send(ctx, http.MethodPost,
				fmt.Sprintf("/journals/%d/attendees/%d/certificate/generate", in.JournalID, in.AttendeeID), nil)
		})

	return s
}

// renewBody to treść POST /certificates/{id}/renew - bez id, które jest w ścieżce.
type renewBody struct {
	CertificateDate string `json:"certificateDate"`
	CourseDateStart string `json:"courseDateStart"`
	CourseDateEnd   string `json:"courseDateEnd,omitempty"`
	LanguageCode    string `json:"languageCode,omitempty"`
}

// updateStudent łączy zmiany z bieżącymi danymi, bo PATCH /students/{id} nadpisuje
// wszystkie pola - pominięte w nim pole zostałoby wyczyszczone.
func updateStudent(ctx context.Context, api *apiClient, in updateStudentInput) ([]byte, error) {
	path := fmt.Sprintf("/students/%d", in.ID)
	raw, err := api.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var current struct {
		Data struct {
			studentFields
			Company *struct {
				ID int64 `json:"id"`
			} `json:"company"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &current); err != nil {
		return nil, fmt.Errorf("nieoczekiwana odpowiedź API: %w", err)
	}
	body := current.Data.studentFields
	if current.Data.Company != nil {
		body.CompanyID = &current.Data.Company.ID
	}

	overrideIfSet(&body.FirstName, in.FirstName)
	overrideIfSet(&body.LastName, in.LastName)
	overrideIfSet(&body.BirthDate, in.BirthDate)
	overrideIfSet(&body.BirthPlace, in.BirthPlace)
	for target, value := range map[**string]*string{
		&body.SecondName: in.SecondName, &body.Pesel: in.Pesel, &body.AddressStreet: in.AddressStreet,
		&body.AddressCity: in.AddressCity, &body.AddressZip: in.AddressZip, &body.Telephone: in.Telephone,
	} {
		if value != nil {
			*target = value
		}
	}
	if in.CompanyID != nil {
		body.CompanyID = in.CompanyID
	}

	return api.send(ctx, http.MethodPatch, path, body)
}

func overrideIfSet(target *string, value *string) {
	if value != nil {
		*target = *value
	}
}

// getJournal składa dziennik z czterech tras, żeby agent dostał w jednym wywołaniu
// identyfikatory sesji i uczestników potrzebne do set_attendance.
func getJournal(ctx context.Context, api *apiClient, id int64) ([]byte, error) {
	parts := map[string]string{
		"journal":    "",
		"sessions":   "/sessions",
		"attendees":  "/attendees",
		"attendance": "/attendance",
	}
	result := make(map[string]json.RawMessage, len(parts))
	for key, suffix := range parts {
		raw, err := api.get(ctx, fmt.Sprintf("/journals/%d%s", id, suffix), nil)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("nieoczekiwana odpowiedź API: %w", err)
		}
		result[key] = envelope.Data
	}
	return json.Marshal(result)
}

func setText(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setInt(q url.Values, key string, value int64) {
	if value > 0 {
		q.Set(key, strconv.FormatInt(value, 10))
	}
}
