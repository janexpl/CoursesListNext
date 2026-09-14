# CoursesList API — przewodnik integracji

Dokument dla agenta lub zespołu budującego aplikację, która komunikuje się z API systemu CoursesList.
Pełny kontrakt (wszystkie operacje, pola, kody błędów) jest w [`openapi.yaml`](./openapi.yaml) — ten plik
opisuje to, czego schemat nie wyraża: model domeny, reguły biznesowe, przepływy i pułapki.

**Przeczytaj sekcje 4 i 5 przed napisaniem pierwszego żądania zapisującego dane.** Opisują zachowania,
które cicho niszczą dane albo dają mylące błędy.

---

## 1. Model domeny

System prowadzi ewidencję szkoleń (głównie BHP) i wystawia zaświadczenia ich ukończenia. UI jest po polsku.

```
Firma ──< Kursant ──< Zaświadczenie >── Kurs ──< Tłumaczenie kursu
                           │              │
                           │              └──< Dziennik szkolenia ──< Sesja
                           │                         │
                           └──── (opcjonalnie) ──────┴──< Uczestnik ──< Obecność (sesja × uczestnik)
```

| Pojęcie | Zasób | Uwagi |
|---|---|---|
| Firma | `companies` | Pracodawca kursantów. NIP unikalny. Może dostawać e-maile o wygasających zaświadczeniach. |
| Kursant | `students` | Opcjonalnie przypisany do jednej firmy. |
| Kurs | `courses` | Symbol unikalny. Zawiera program szkolenia i szablon HTML zaświadczenia (+ tłumaczenia). |
| Zaświadczenie | `certificates` | Wystawiane kursantowi z kursu. Ma numer rejestru `numer/SYMBOL/rok`, unikalny w obrębie (kurs, rok). |
| Dziennik szkolenia | `journals` | Dokumentacja jednej edycji kursu: sesje, uczestnicy, obecność, skany. Status `draft` → `closed`. |
| Uczestnik | `journals/{id}/attendees` | Kursant dodany do dziennika. **Ma własne `id`, różne od `studentId`.** |

### Kopie danych (snapshoty)

Zaświadczenie i uczestnik dziennika zapisują **kopię** danych z chwili utworzenia:

- zaświadczenie — imię, nazwisko, datę i miejsce urodzenia, PESEL i firmę kursanta oraz nazwę, symbol, program,
  szablon i okres ważności kursu;
- uczestnik — imię i nazwisko, datę urodzenia i nazwę firmy.

Późniejsza zmiana kursanta, firmy lub kursu **nie zmienia** wystawionych zaświadczeń ani uczestników.
Wyjątek: `PATCH /certificates/{id}` odświeża kopię danych kursanta.

---

## 2. Uwierzytelnianie

### Klucz API

Każde żądanie (poza `GET /healthz`) wysyła nagłówek:

```http
Authorization: Bearer clk_...
```

Klucze wystawia administrator w aplikacji webowej: **Administracja → Klucze API**. Klucz pokazywany jest
tylko raz, przy utworzeniu — przechowuj go jako sekret (zmienna środowiskowa, sejf), nigdy w repozytorium.
API **nie pozwala** wystawiać ani odwoływać kluczy za pomocą klucza.

Każdy klucz:

- działa w imieniu **konta serwisowego** (zwykłego użytkownika systemu) — to konto widnieje w historii zmian
  jako autor operacji wykonanych kluczem;
- ma listę **zakresów** ograniczających dostępne operacje;
- może mieć datę wygaśnięcia i może zostać unieważniony w dowolnej chwili.

Pierwsze wywołanie integracji powinno sprawdzić klucz:

```http
GET /api/v1/auth/me
Authorization: Bearer clk_...
```

Zwraca konto serwisowe (`role: 1` = administrator). Nie zwraca listy zakresów — brak zakresu wyjdzie dopiero
jako 403 przy konkretnej operacji.

### Zakresy

Format `<zasób>:<akcja>`. **Zakres `write` obejmuje `read`** tego samego zasobu.

| Zakres | Pozwala na |
|---|---|
| `students:read` / `students:write` | kursanci; także `GET /companies/{id}/students` |
| `companies:read` / `companies:write` | firmy; także wyszukiwanie w GUS (`lookup-by-nip`) |
| `courses:read` / `courses:write` | kursy z treścią i tłumaczeniami |
| `certificates:read` / `certificates:write` | zaświadczenia i PDF; także listy zaświadczeń kursanta, firmy i kursu |
| `journals:read` / `journals:write` | dzienniki, sesje, uczestnicy, obecność, skany, PDF; także generowanie zaświadczenia uczestnikowi |
| `registries:read` | kolejny wolny numer rejestru |
| `dashboard:read` | statystyki i wygasające zaświadczenia |
| `users:read` / `users:write` | zarządzanie użytkownikami **(wymaga konta administratora)** |
| `audit-log:read` | historia zmian **(wymaga konta administratora)** |

Wymagany zakres każdej operacji: pole `x-required-scope` w `openapi.yaml`. Operacje z `x-requires-admin: true`
wymagają **dodatkowo**, żeby konto serwisowe klucza było administratorem — sam zakres nie wystarczy.

Zakresy przyznaje administrator; aplikacja powinna prosić tylko o te, których naprawdę używa.

### Rozpoznawanie odmów

| Status | `error.message` | Co zrobić |
|---|---|---|
| 401 | `missing credentials` | Brak nagłówka `Authorization`. |
| 401 | `invalid or expired api key` | Klucz nieznany, unieważniony, wygasły albo konto usunięte. **Nie ponawiaj** — potrzebny nowy klucz. |
| 403 | `api key is missing the <zakres> scope` | Poproś administratora o ten zakres. |
| 403 | `admin access required` | Konto serwisowe nie jest administratorem. |
| 403 | `this endpoint requires an interactive session` | Operacja dostępna tylko z przeglądarki (patrz niżej). |

### Niedostępne dla klucza API

Tylko po zalogowaniu w przeglądarce, celowo pominięte w specyfikacji: `POST /auth/login`, `POST /auth/logout`,
`PATCH /account/profile`, `PATCH /account/password`, całe `/admin/api-keys`. Trasa
`/internal/notifications/*` używa osobnego tokenu usługi powiadomień i nie jest częścią tego API.

---

## 3. Konwencje

### Adres

```
{baseUrl}/api/v1/...
```

Lokalnie `http://localhost:8081`. Wołaj Go API bezpośrednio. Konkretny adres produkcyjny ustal z administratorem
systemu — nie jest częścią repozytorium.

### Koperta odpowiedzi

```json
{ "data": { ... } }
{ "data": [ ... ] }
{ "data": [ ... ], "pagination": { "page": 1, "limit": 10, "total": 42, "totalPages": 5 } }
```

Błąd:

```json
{ "error": { "code": "bad_request", "message": "invalid request body" } }
```

`code` ∈ `bad_request`, `unauthorized`, `forbidden`, `not_found`, `conflict`, `internal_error`.
Komunikaty wymienione w `openapi.yaml` przy konkretnych operacjach są stabilne — można na nich opierać logikę.
**Komunikaty są po angielsku; tekst dla użytkownika końcowego buduj po swojej stronie.**

Wyjątki od koperty: `GET .../pdf` i pobieranie skanów zwracają plik binarny; część operacji zwraca `204` bez treści.

### Formaty danych

| Typ | Format | Przykład |
|---|---|---|
| Data | `YYYY-MM-DD` | `2026-09-14` |
| Znacznik czasu | `YYYY-MM-DD HH:MM:SS`, **czas lokalny serwera, bez strefy** | `2026-09-14 10:30:00` |
| Identyfikator | liczba całkowita dodatnia | `128` |
| Numer zaświadczenia | `numer/SYMBOL/rok` (wyliczany, nie ma pola) | `12/BHP/2026` |

Znaczniki czasu **nie są ISO 8601** — parsuj je jawnym formatem.

### Pola opcjonalne

W odpowiedziach brakująca wartość to `null`. Jedyny wyjątek: `companyName` na listach zaświadczeń i na pulpicie
to pusty string, gdy kursant nie miał firmy.
W żądaniach pusty string w polu opcjonalnym jest zapisywany jako `null` (białe znaki na brzegach są obcinane).

### Ciała żądań

- `Content-Type: application/json`, maks. **1 MiB** (poza wgrywaniem skanów: `multipart/form-data`, maks. 16 MiB).
- **Pole spoza schematu → 400 `invalid request body`.** Nie wysyłaj `id`, pól tylko do odczytu ani pól
  „na zapas". Nie da się więc odesłać obiektu pobranego przez GET bez przefiltrowania.

### Listy, wyszukiwanie, limity

- Listy przyjmują `search` i `limit` (1–100, domyślnie 50).
- **Brak paginacji** poza `GET /companies/{id}/certificates` i `GET /courses/{id}/certificates`
  (`page`, `limit` 1–100 z domyślną wartością **10**).
- Konsekwencja: **nie da się pobrać pełnej listy kursantów, firm, kursów, zaświadczeń ani dzienników,
  jeśli jest ich więcej niż 100.** Zawężaj wyszukiwaniem (`search`, `companyId`, `courseId`, `status`,
  zakresy dat). Nie projektuj pełnej synchronizacji bazy przez to API.
- Kolejność wyników każdej listy jest opisana w `openapi.yaml` i jest deterministyczna.

### Limity techniczne

- Limit czasu żądania: **60 s** (dotyczy zwłaszcza generowania PDF).
- Brak limitu liczby żądań dla kluczy API — mimo to nie wysyłaj setek żądań równolegle; baza jest współdzielona
  z użytkownikami aplikacji webowej. Rozsądnie: do kilku żądań jednocześnie.
- CORS dopuszcza nagłówek `Authorization`, ale tylko dla skonfigurowanych domen. Integracja serwer-serwer
  CORS nie dotyczy. **Nie umieszczaj klucza API w kodzie działającym w przeglądarce.**

---

## 4. `PATCH` oznacza pełne nadpisanie

**To najważniejsza reguła tego API.** Wszystkie operacje `PATCH` zapisujące obiekt działają jak `PUT`:
pole pominięte w żądaniu **nie zostaje bez zmian**, tylko jest czyszczone. Jedyny wyjątek to tłumaczenia kursu (niżej).

| Operacja | Co zniknie, jeśli pole pominiesz |
|---|---|
| `PATCH /students/{id}` | `secondName`, `pesel`, adres, `telephone`, `companyId` (kursant straci firmę) |
| `PATCH /companies/{id}` | `email`, `contactPerson`, `note`, `expiryNotificationEmail`; `expiryNotificationsEnabled` → `false` |
| `PATCH /courses/{id}` | — (wszystkie pola kursu wymagane). **Wyjątek:** pominięte `certificateTranslations` zostawia tłumaczenia bez zmian; podana lista je zastępuje, a `[]` usuwa wszystkie |
| `PATCH /certificates/{id}` | `courseDateEnd` |
| `PATCH /journals/{id}` | `companyId`, `organizerAddress`, `notes` |
| `PATCH /admin/users/{id}` | — (wszystkie pola wymagane) |

Bezpieczny wzorzec aktualizacji:

1. `GET` obiektu.
2. Zbuduj ciało żądania **ze wszystkimi polami schematu `*Write`/`*Update`**, zmieniając tylko to, co trzeba.
3. Odrzuć pola tylko do odczytu (`id`, obiekty zagnieżdżone typu `company`) — inaczej 400.
4. `PATCH`.

Mapowanie odpowiedzi GET na ciało PATCH nie jest 1:1 — przykłady:

| Odpowiedź GET | Ciało PATCH |
|---|---|
| `StudentDetails.company.id` | `StudentWrite.companyId` |
| `CourseDetails.certificateTranslations` | `CourseWrite.certificateTranslations` (ten sam kształt — przepisz w całości) |
| `JournalDetails.companyId` | `JournalUpdate.companyId` (bez `courseId` — kursu nie da się zmienić) |

Operacje `PATCH` o innej semantyce (nie nadpisują obiektu): `PATCH /journals/{id}/sessions/{sessionId}`
(zmienia tylko datę i prowadzącego), `PATCH /journals/{id}/attendance` (upsert jednego wpisu),
`PATCH /journals/{id}/attendees/{attendeeId}/certificate`, `PATCH /admin/users/{id}/password`.

---

## 5. Zachowania, o których trzeba wiedzieć

Poniższe zachowania są zamierzone — nie są błędami do obejścia, ale łatwo je przeoczyć.

### Zaświadczenia

1. `studentName` na liście zaświadczeń to imię i nazwisko. W szczegółach dane są rozbite na
   `studentFirstname`, `studentSecondname` i `studentLastname`.
2. `DELETE /certificates/{id}` wymaga konta administratora. Usunięte zaświadczenie daje później 404 na GET i PDF,
   a jego numer rejestru można wykorzystać ponownie.
3. PESEL kursanta **nie jest walidowany** — pole przechowuje też numery dokumentów cudzoziemców
   (w obecnych danych większość wartości nie jest poprawnym PESEL-em). Nie odrzucaj takich wartości po swojej stronie.

### Kursy

4. `courseProgram` to **string zawierający JSON**, nie zagnieżdżony obiekt:
   `"[{\"Subject\":\"Przepisy BHP\",\"TheoryTime\":\"4\",\"PracticeTime\":\"0\"}]"`.
   Musi być tablicą JSON (inaczej 400 `course program must be a JSON array`); klucze wielką literą, godziny jako stringi.
5. Tłumaczenia tylko w językach `en`, `de`, `uk`, `cs`, `sk`, `lt`; `pl` to język bazowy.
   Przy `PATCH /courses/{id}` pominięte `certificateTranslations` zostawia je bez zmian, a `[]` usuwa wszystkie (sekcja 4).
6. `expiryTime` równe `0` to poprawny okres ważności (kończy się z końcem kursu). Kurs bez terminu ważności ma `null` —
   nie sprawdzaj go warunkiem „prawdziwości" (`if (expiryTime)`), bo pomylisz `0` z brakiem terminu.

### Kursanci i firmy

7. NIP jest walidowany przy zapisie firmy (400 `nip validation error: …`, te same komunikaty co w
   `GET /companies/lookup-by-nip`) i zapisywany jako same cyfry — `123-456-32-18` wróci jako `1234563218`.
8. `expiryNotificationEmail` to jeden string z adresami rozdzielonymi przecinkami (maks. 10), nie tablica.
9. Brak operacji usuwania kursantów, firm i kursów.

### Dzienniki

10. **`POST /journals` od razu tworzy sesje** z programu kursu (maks. 8 godzin dziennie, kolejne dni od `dateStart`);
    ich liczbę podaje `sessionsCount`. Jeśli sesje nie mieszczą się w zakresie dat, API zwraca 400
    `course program does not fit within journal dates` i **nie tworzy dziennika** — wydłuż `dateEnd` i ponów.
    `POST .../sessions/generate-from-course` zwykle zwraca wtedy 409, bo sesje już istnieją.
11. W ścieżkach `/attendees/{attendeeId}` i w `journalAttendeeId` podajesz **id uczestnika**, nie id kursanta.
12. Zamknięty dziennik blokuje (409 `journal is closed`): zmianę nagłówka i sesji, obecność, dodawanie i usuwanie uczestników.
    **Nie blokuje** wgrywania skanów (podpisany dziennik skanuje się po zamknięciu), wystawiania i powiązywania
    zaświadczeń ani usunięcia całego dziennika.
13. `DELETE /journals/{id}` usuwa trwale sesje, uczestników, obecność i skany (także dla zamkniętego dziennika).
    Zaświadczenia zostają.

### Pozostałe

14. Historia zmian (`.../audit-log`) nieistniejącego obiektu to pusta lista, nie 404 — dzięki temu historia pozostaje
    dostępna np. po usunięciu użytkownika. Pozostałe listy podrzędne dla nieistniejącego rodzica zwracają 404.
15. Unikalność adresu e-mail użytkownika **rozróżnia wielkość liter**: `Jan@example.com` i `jan@example.com`
    to dla API dwa różne adresy. Normalizuj adresy po swojej stronie, zanim utworzysz konto.

---

## 6. Typowe przepływy

### 6.1. Rejestracja firmy z danymi z GUS

```http
GET /api/v1/companies/lookup-by-nip?nip=1234563218            # companies:read
```

Zmapuj odpowiedź na `CompanyWrite`: `name`, `city`, `postalCode` → `zipcode`, `street` + `houseNumber`
(+ `/apartment`, jeśli niepuste) → `street`. `telephone` jest wymagany, a GUS go nie zwraca — pozyskaj go osobno.

```http
POST /api/v1/companies                                         # companies:write
{ "name": "...", "street": "...", "city": "...", "zipcode": "...", "nip": "1234563218",
  "telephone": "...", "expiryNotificationsEnabled": false }
```

409 = firma już istnieje → znajdź ją przez `GET /companies?search=1234563218`. 400 `nip validation error: …` = niepoprawny NIP;
`lookup-by-nip` stosuje tę samą walidację, więc NIP, który przeszedł wyszukiwanie w GUS, przejdzie też zapis.

### 6.2. Wystawienie zaświadczenia ręcznie

```http
GET  /api/v1/courses/{courseId}                                # courses:read    — dostępne tłumaczenia
GET  /api/v1/registries/next-number?courseId={courseId}&year=2026   # registries:read
POST /api/v1/certificates                                      # certificates:write
{
  "studentId": 15,
  "courseId": 128,
  "certificateDate": "2026-09-14",
  "courseDateStart": "2026-09-10",
  "courseDateEnd": "2026-09-12",
  "registryYear": 2026,
  "registryNumber": 43,
  "languageCode": "pl"
}
→ 201 { "data": { "id": 9812 } }
GET  /api/v1/certificates/9812                                 # certificates:read
GET  /api/v1/certificates/9812/pdf                             # plik PDF
```

Reguły walidacji:

- `certificateDate` ≥ `courseDateEnd` (albo `courseDateStart`, gdy brak końca), inaczej 400
  `certificate date cannot be before course end date`.
- `courseDateEnd` ≥ `courseDateStart`, inaczej 400 `invalid certificate data`.
- **Chronologia rejestru**: data zaświadczenia musi mieścić się między datami zaświadczeń o sąsiednich
  numerach w tym kursie i roku. Nadanie numeru 43 z datą wcześniejszą niż zaświadczenie nr 42 → 400
  `invalid certificate data`. Najbezpieczniej: zawsze kolejny numer i data nie wcześniejsza niż ostatnie zaświadczenie.
- Numer zajęty → 409. `next-number` to podpowiedź, nie rezerwacja — przy 409 pobierz numer ponownie i powtórz,
  z ograniczoną liczbą prób.
- `languageCode` ≠ `pl` wymaga tłumaczenia kursu w tym języku, inaczej 400 `certificate translation not found`.
- Nieistniejący kursant lub kurs → 404 `student not found` / `course not found`.

### 6.3. Dziennik szkolenia od utworzenia do zaświadczeń

Wszystkie kroki: `journals:write` (odczyty `journals:read`).

```http
POST  /api/v1/journals
{ "courseId": 128, "companyId": 7, "title": "Szkolenie okresowe BHP — wrzesień",
  "organizerName": "Jan Kowalski", "location": "Żyrardów", "formOfTraining": "stacjonarna",
  "legalBasis": "...", "dateStart": "2026-09-10", "dateEnd": "2026-09-12" }
→ 201, id = 55   (sesje już wygenerowane)

GET   /api/v1/journals/55/sessions                  → id sesji
POST  /api/v1/journals/55/attendees   { "studentId": 15 }   → 201, data.id = id uczestnika (np. 301)
PATCH /api/v1/journals/55/attendance  { "journalSessionId": 900, "journalAttendeeId": 301, "present": true }
      ... powtórz dla każdej pary (sesja, uczestnik)
POST  /api/v1/journals/55/attendees/301/certificate/generate   → 201, data.id = id zaświadczenia
POST  /api/v1/journals/55/attendance-scan   (multipart, pole "file")
POST  /api/v1/journals/55/close
GET   /api/v1/journals/55/pdf
```

Generowanie zaświadczenia z dziennika samo dobiera numer i datę (rok i data z `dateEnd` dziennika, numer = ostatni + 1)
i od razu wiąże zaświadczenie z uczestnikiem. Nie wymaga zakresu `certificates:write`.

### 6.4. Wygasające zaświadczenia

```http
GET /api/v1/dashboard                               # dashboard:read — do 50 najbliższych w ciągu 30 dni
GET /api/v1/certificates?dateFrom=2021-01-01&dateTo=2021-12-31&limit=100   # certificates:read
```

Pole `expiryDate` jest wyliczane (`courseDateEnd` + lata ważności × 365 dni) i nie da się po nim filtrować w API.

---

## 7. Obsługa błędów — zalecenia

| Status | Ponawiać? | Uwagi |
|---|---|---|
| 400 | Nie | Błąd po stronie aplikacji. Ogólne `invalid request body` nie wskazuje pola (brak wymaganego pola, pole spoza schematu, zła data) — sprawdź sekcje 4 i 5 oraz schemat; walidacja NIP-u, programu kursu i tłumaczeń zwraca konkretne komunikaty. |
| 401 | Nie | Klucz nieważny — zatrzymaj integrację i zgłoś potrzebę nowego klucza. |
| 403 | Nie | Brak zakresu lub uprawnień administratora. |
| 404 | Nie | — |
| 409 | Zależy | Konflikt stanu. Dla numeru rejestru: pobierz nowy numer i ponów. |
| 500 | Ostrożnie | Błąd serwera. Ponów najwyżej raz, z opóźnieniem; operacje `POST` nie są idempotentne. |
| Brak odpowiedzi / timeout | Ostrożnie | `POST` mógł zostać wykonany — przed ponowieniem sprawdź, czy obiekt nie powstał. |

API nie obsługuje kluczy idempotencji.

---

## 8. Czego API nie oferuje

- webhooków ani powiadomień o zmianach — zmiany trzeba odpytywać;
- filtrowania „zmienione od" (`updatedSince`) i paginacji dla dużych list;
- operacji zbiorczych (np. obecność wielu osób jednym żądaniem);
- usuwania kursantów, firm i kursów;
- wersjonowania poza prefiksem `/api/v1` — specyfikacja opisuje stan kodu z gałęzi `api_and_webhook`
  i nie jest gwarancją zgodności na przyszłość.

Jeśli integracja potrzebuje którejś z tych rzeczy, zgłoś to właścicielowi API zamiast obchodzić ograniczenie.
