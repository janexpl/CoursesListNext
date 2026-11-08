# Serwer MCP CoursesList

Pozwala agentowi AI (Claude Code, Claude Desktop i inne klienty MCP) obsługiwać system:
wyszukiwać kursantów i firmy, wystawiać i przedłużać zaświadczenia, zakładać dzienniki
i zaznaczać obecności.

Serwer działa lokalnie (transport stdio) i woła REST API kluczem API. Nie ma dostępu do bazy
ani własnej logiki - walidacja, numeracja w rejestrze, uprawnienia i historia zmian działają
tak samo jak dla każdej innej integracji. W historii zmian operacje agenta są widoczne jako
konto serwisowe klucza.

## Klucz API

W panelu: **Administracja → Klucze API** załóż klucz dla agenta. Zakresy decydują, co agent
może zrobić - API odrzuci wszystko poza nimi. Pełna obsługa narzędzi:

`students:write`, `companies:write`, `courses:read`, `certificates:write`, `journals:write`,
`dashboard:read`

Agent tylko do odczytu: te same zasoby z `:read`.

## Budowanie

```bash
cd mcp
go build -o ~/bin/courseslist-mcp ./cmd/mcp
```

## Konfiguracja

- `COURSESLIST_API_URL` - adres systemu, pod którym działa `/api/v1`, np. `https://kursy.example.pl`
- `COURSESLIST_API_KEY` - klucz `clk_...`

Claude Code:

```bash
claude mcp add courseslist \
  -e COURSESLIST_API_URL=https://kursy.example.pl \
  -e COURSESLIST_API_KEY=clk_... \
  -- ~/bin/courseslist-mcp
```

Claude Desktop (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "courseslist": {
      "command": "/Users/ja/bin/courseslist-mcp",
      "env": {
        "COURSESLIST_API_URL": "https://kursy.example.pl",
        "COURSESLIST_API_KEY": "clk_..."
      }
    }
  }
}
```

Klucz trzymaj tylko w konfiguracji klienta, nie w repozytorium.

## Narzędzia

| Obszar | Odczyt | Zapis |
|---|---|---|
| Kursanci | `search_students`, `get_student`, `list_student_certificates` | `create_student`, `update_student` |
| Firmy | `search_companies`, `get_company`, `lookup_company_by_nip` (GUS) | `create_company` |
| Kursy | `list_courses`, `get_course` | - |
| Zaświadczenia | `search_certificates`, `get_certificate`, `get_dashboard` (wygasające) | `issue_certificate`, `renew_certificate` |
| Dzienniki | `list_journals`, `get_journal` (z sesjami, uczestnikami i obecnościami) | `create_journal`, `add_journal_attendee`, `set_attendance`, `generate_attendee_certificate` |

Narzędzia zapisujące są oznaczone w protokole jako zmieniające dane, więc klient pyta
o zgodę przed ich wykonaniem.

- `update_student` zmienia tylko podane pola: pobiera bieżące dane i odsyła je z poprawkami,
  bo `PATCH /students/{id}` nadpisuje wszystkie pola.
- `issue_certificate` i `renew_certificate` wysyłają `Idempotency-Key` wyliczony z treści.
  Ponowienie tego samego wywołania zwraca pierwsze zaświadczenie zamiast wystawiać drugie.
- Błąd API (np. „Program kursu nie mieści się w zakresie dat dziennika”) wraca do agenta jako
  błąd narzędzia z komunikatem API.

Celowo poza zakresem: administracja (użytkownicy, klucze, nadruki, podstawy prawne) oraz
operacje nieodwracalne - unieważnianie zaświadczeń, usuwanie i zamykanie dzienników.

## Testy

```bash
cd mcp
go test ./...
```
