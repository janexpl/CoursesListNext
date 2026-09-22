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
| Firma | `companies` | Pracodawca kursantów. NIP unikalny. Może mieć `externalId` platformy. Może dostawać e-maile o wygasających zaświadczeniach. |
| Kursant | `students` | Opcjonalnie przypisany do jednej firmy. Może mieć `externalId` platformy. |
| Kurs | `courses` | Symbol unikalny. Zawiera program szkolenia i szablon HTML zaświadczenia (+ tłumaczenia). Flaga `deliveredByPlatform` wyznacza katalog platformy. |
| Zaświadczenie | `certificates` | Wystawiane kursantowi z kursu. Ma numer rejestru `numer/SYMBOL/rok`, unikalny w obrębie (kurs, rok), i kod weryfikacyjny. Może zostać unieważnione; wtórnik to ten sam dokument z adnotacją „DUPLIKAT". |
| Dziennik szkolenia | `journals` | Dokumentacja jednej edycji kursu: sesje, uczestnicy, obecność, skany. Status `draft` → `closed`. |
| Uczestnik | `journals/{id}/attendees` | Kursant dodany do dziennika. **Ma własne `id`, różne od `studentId`.** |

### Kopie danych (snapshoty)

Zaświadczenie i uczestnik dziennika zapisują **kopię** danych z chwili utworzenia:

- zaświadczenie — imię, nazwisko, datę i miejsce urodzenia, PESEL i firmę kursanta oraz nazwę, symbol, program,
  szablon i okres ważności kursu;
- uczestnik — imię i nazwisko, datę urodzenia i nazwę firmy.

Późniejsza zmiana kursanta, firmy lub kursu **nie zmienia** wystawionych zaświadczeń ani uczestników.
Wyjątek: `PATCH /certificates/{id}` odświeża kopię danych kursanta. Wystawienie duplikatu
(`POST /certificates/{id}/duplicate`) nie zmienia treści dokumentu — dokłada tylko adnotację o wtórniku.

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

Znaczniki czasu **nie są ISO 8601** — parsuj je jawnym formatem. Wyjątki, celowo w ISO 8601: parametr
`updatedSince` (sekcja 6.6) i `timestamp` w webhookach (sekcja 6.7).

### Pola opcjonalne

W odpowiedziach brakująca wartość to `null`. Wyjątki: `companyName` na listach zaświadczeń i na pulpicie
to pusty string, gdy kursant nie miał firmy, a `telephone` firmy to pusty string, gdy firma nie ma telefonu.
W żądaniach pusty string w polu opcjonalnym jest zapisywany jako `null` (białe znaki na brzegach są obcinane).

### Ciała żądań

- `Content-Type: application/json`, maks. **1 MiB** (poza wgrywaniem skanów: `multipart/form-data`, maks. 16 MiB).
- **Pole spoza schematu → 400 `invalid request body`.** Nie wysyłaj `id`, pól tylko do odczytu ani pól
  „na zapas". Nie da się więc odesłać obiektu pobranego przez GET bez przefiltrowania.

### Listy, wyszukiwanie, limity

- Listy przyjmują `search` i `limit` (1–100, domyślnie 50).
- Paginację (`page` od 1 + koperta `pagination`) mają tylko:
  `GET /companies/{id}/certificates` i `GET /courses/{id}/certificates` (`limit` 1–100, domyślnie **10**) oraz
  `GET /courses` i `GET /courses/details` (`limit` 1–100, domyślnie **50**).
- Konsekwencja: **nie da się pobrać pełnej listy kursantów, firm, zaświadczeń ani dzienników,
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
| `PATCH /companies/{id}` | `email`, `contactPerson`, `note`, `expiryNotificationEmail`, `telephone` (→ pusty string); `expiryNotificationsEnabled` → `false` |
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

`PUT /students/by-external-id/{externalId}` i `PUT /companies/by-external-id/{externalId}` nadpisują istniejący
rekord dokładnie tak samo jak odpowiadający im `PATCH`. Żadna z tych operacji nie zmienia `externalId`.

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
   a jego numer rejestru można wykorzystać ponownie (poza unieważnionym — niżej). **Do formalnego wycofania
   dokumentu służy unieważnienie, nie usunięcie.**
3. `POST /certificates/{id}/revoke` `{ "reason": "..." }` unieważnia dokument: zostaje w rejestrze, w GET i na
   listach z `revokedAt` (w szczegółach też `revokeReason`), a jego numer jest zajęty **na zawsze** — także po
   `DELETE`. Ponowne unieważnienie → 409 `certificate already revoked`. Unieważnionego dokumentu nie da się
   zmienić (`PATCH` → 409 `certificate is revoked`) ani zduplikować.
   **Decyzja dot. PDF:** `GET /certificates/{id}/pdf` dla unieważnionego dokumentu zwraca **409
   `certificate is revoked`** — API nie generuje wydruku unieważnionego dokumentu (ani ze znakiem wodnym),
   żeby nie krążył plik wyglądający na ważny. Stan i dane dokumentu są dostępne w `GET /certificates/{id}`.
4. `POST /certificates/{id}/duplicate` `{ "reason": "..." }` odnotowuje wystawienie duplikatu (wtórnika).
   **To ten sam dokument, nie nowy**: numer rejestru, kod weryfikacyjny, data wystawienia i treść zostają bez
   zmian, dochodzą tylko `duplicateIssuedAt` i `duplicateReason`, a wydruk PDF dostaje adnotację „DUPLIKAT"
   z datą wystawienia wtórnika. Odpowiedź `200` z tym samym dokumentem. Duplikat nie zajmuje kolejnego numeru
   w rejestrze i nie wpływa na ważność.
   Operację można powtórzyć (kursant może zgubić dokument ponownie) — liczy się data ostatniego wystawienia,
   a każde zostaje w historii zmian. Unieważnionego dokumentu nie da się zduplikować (409 `certificate is revoked`).
5. Przypomnienia o wygasaniu (pulpit, `/internal/notifications/expiring-certificates`) pomijają dokumenty
   unieważnione. Wystawienie wtórnika niczego tu nie zmienia — to nadal ten sam dokument.
6. Każde zaświadczenie ma `verificationCode` — 12 znaków z alfabetu bez `0`, `O`, `1`, `I`, `l`, losowy, unikalny
   i niezmienny. Dostają go wszystkie dokumenty (API, aplikacja webowa, dziennik, także te sprzed wprowadzenia kodu).
   Zwracany w `CertificateDetails` i w odpowiedzi na `POST /certificates`. Zaświadczenie po kodzie:
   `GET /certificates/by-verification-code/{code}` (`certificates:read`, wielkość liter bez znaczenia).
   Na wydruku PDF kod pojawia się jako **kod QR** prowadzący do publicznej strony weryfikacji
   (`GET /public/certificates/{code}`, sekcja 6.8), podpisany „Sprawdź ważność" — samego kodu
   tekstem na dokumencie nie ma.
7. PESEL kursanta **nie jest walidowany** — pole przechowuje też numery dokumentów cudzoziemców
   (w obecnych danych większość wartości nie jest poprawnym PESEL-em). Nie odrzucaj takich wartości po swojej stronie.

### Kursy

8. `courseProgram` to **string zawierający JSON**, nie zagnieżdżony obiekt:
   `"[{\"Subject\":\"Przepisy BHP\",\"TheoryTime\":\"4\",\"PracticeTime\":\"0\"}]"`.
   Musi być tablicą JSON (inaczej 400 `course program must be a JSON array`); klucze wielką literą, godziny jako stringi.
9. Tłumaczenia tylko w językach `en`, `de`, `uk`, `cs`, `sk`, `lt`; `pl` to język bazowy.
   Przy `PATCH /courses/{id}` pominięte `certificateTranslations` zostawia je bez zmian, a `[]` usuwa wszystkie (sekcja 4).
10. `expiryTime` równe `0` to poprawny okres ważności (kończy się z końcem kursu). Kurs bez terminu ważności ma `null` —
   nie sprawdzaj go warunkiem „prawdziwości" (`if (expiryTime)`), bo pomylisz `0` z brakiem terminu.
11. `GET /courses` i `GET /courses/details` przyjmują `updatedSince` i `deliveredByPlatform` (sekcja 6.6).
   Wartość `deliveredByPlatform` jest na elementach `GET /courses` i pod `GET /courses/{id}/platform-delivery`,
   ale **nie** w `CourseDetails` (także nie w `GET /courses/details`) — kształt `CourseDetails` się nie zmienia.

### Kursanci i firmy

12. NIP jest walidowany przy zapisie firmy (400 `nip validation error: …`, te same komunikaty co w
   `GET /companies/lookup-by-nip`) i zapisywany jako same cyfry — `123-456-32-18` wróci jako `1234563218`.
13. `expiryNotificationEmail` to jeden string z adresami rozdzielonymi przecinkami (maks. 10), nie tablica.
14. Brak operacji usuwania kursantów, firm i kursów.
15. Imię, nazwisko i data urodzenia identyfikują osobę — nie da się utworzyć drugiego kursanta o tych samych
    wartościach, także różniących się tylko wielkością liter lub spacjami (409). Przed utworzeniem kursanta
    wyszukaj go (`GET /students?search=...`). Dane sprzed wprowadzenia tej reguły mogą zawierać takie duplikaty;
    edycja rekordu z takiej pary, zmieniająca jego zapis na identyczny z bliźniakiem, też zwróci 409.
    Integracje, które mają własny identyfikator osoby, powinny zamiast tego używać
    `PUT /students/by-external-id/{externalId}` (sekcja 6.2).
16. `externalId` kursanta i firmy (maks. 64 znaki, unikalny) ustawia wyłącznie `PUT .../by-external-id/{externalId}`.
    Jest tylko do odczytu w `StudentDetails`/`CompanyDetails`, nie ma go na listach, a `POST` i `PATCH` go nie
    przyjmują (400) i nie zmieniają. Rekordy zakładane w aplikacji webowej mają `externalId: null`.
17. `telephone` firmy jest opcjonalny. Brak telefonu zapisuje się i wraca jako pusty string (ok. połowa istniejących
    firm nie ma telefonu).

### Dzienniki

18. **`POST /journals` od razu tworzy sesje** z programu kursu (maks. 8 godzin dziennie, kolejne dni od `dateStart`);
    ich liczbę podaje `sessionsCount`. Jeśli sesje nie mieszczą się w zakresie dat, API zwraca 400
    `course program does not fit within journal dates` i **nie tworzy dziennika** — wydłuż `dateEnd` i ponów.
    `POST .../sessions/generate-from-course` zwykle zwraca wtedy 409, bo sesje już istnieją.
19. W ścieżkach `/attendees/{attendeeId}` i w `journalAttendeeId` podajesz **id uczestnika**, nie id kursanta.
20. Zamknięty dziennik blokuje (409 `journal is closed`): zmianę nagłówka i sesji, obecność, dodawanie i usuwanie uczestników.
    **Nie blokuje** wgrywania skanów (podpisany dziennik skanuje się po zamknięciu), wystawiania i powiązywania
    zaświadczeń ani usunięcia całego dziennika.
21. `DELETE /journals/{id}` usuwa trwale sesje, uczestników, obecność i skany (także dla zamkniętego dziennika).
    Zaświadczenia zostają.

### Pozostałe

22. Historia zmian (`.../audit-log`) nieistniejącego obiektu to pusta lista, nie 404 — dzięki temu historia pozostaje
    dostępna np. po usunięciu użytkownika. Pozostałe listy podrzędne dla nieistniejącego rodzica zwracają 404.
23. Unikalność adresu e-mail użytkownika **rozróżnia wielkość liter**: `Jan@example.com` i `jan@example.com`
    to dla API dwa różne adresy. Normalizuj adresy po swojej stronie, zanim utworzysz konto.

---

## 6. Typowe przepływy

### 6.1. Rejestracja firmy z danymi z GUS

```http
GET /api/v1/companies/lookup-by-nip?nip=1234563218            # companies:read
```

Zmapuj odpowiedź na `CompanyWrite`: `name`, `city`, `postalCode` → `zipcode`, `street` + `houseNumber`
(+ `/apartment`, jeśli niepuste) → `street`. GUS nie zwraca telefonu — `telephone` jest opcjonalny, możesz go pominąć.

```http
POST /api/v1/companies                                         # companies:write
{ "name": "...", "street": "...", "city": "...", "zipcode": "...", "nip": "1234563218",
  "expiryNotificationsEnabled": false }
```

Integracja z własnym identyfikatorem firmy: zamiast `POST` użyj `PUT /api/v1/companies/by-external-id/{externalId}`
z tym samym ciałem (sekcja 6.2).

409 = firma już istnieje → znajdź ją przez `GET /companies?search=1234563218`. 400 `nip validation error: …` = niepoprawny NIP;
`lookup-by-nip` stosuje tę samą walidację, więc NIP, który przeszedł wyszukiwanie w GUS, przejdzie też zapis.

### 6.2. Znajdź-lub-utwórz firmę i kursanta po identyfikatorze platformy

Przepływ integracji wydającej zaświadczenia maszynowo: firma → kursant → zaświadczenie (sekcja 6.3).
Każdy krok można powtarzać bez tworzenia duplikatów.

```http
PUT /api/v1/companies/by-external-id/0b8e6f5a-...          # companies:write
{ "name": "...", "street": "...", "city": "...", "zipcode": "...", "nip": "1234563218" }
→ 201 { "data": { "id": 7, ..., "externalId": "0b8e6f5a-..." } }     (kolejne wywołania: 200)

PUT /api/v1/students/by-external-id/3f6c1a2e-...           # students:write
{ "firstName": "Anna", "lastName": "Nowak", "birthDate": "1991-05-20", "birthPlace": "Kraków", "companyId": 7 }
→ 201 { "data": { "id": 15, ..., "externalId": "3f6c1a2e-..." } }    (kolejne wywołania: 200)
```

- Brak rekordu o tym `externalId` → `201` i nowy rekord. Rekord istnieje → `200` i **pełne nadpisanie** jak w PATCH
  (sekcja 4) — wysyłaj komplet danych.
- Ciało to zwykłe `StudentWrite` / `CompanyWrite`; `externalId` jest tylko w ścieżce. `birthDate` i `birthPlace`
  kursanta pozostają wymagane — trafiają na zaświadczenie.
- `externalId`: 1–64 znaki, porównywany dokładnie, nie jest interpretowany. Zakoduj go w ścieżce; `/` nie jest obsługiwany.
- Równoległe wywołania z tym samym `externalId` tworzą jeden rekord (jedno `201`, pozostałe `200`).
- **Kolizja klucza naturalnego z innym rekordem** (kursant: imię + nazwisko + data urodzenia bez rozróżniania
  wielkości liter i spacji; firma: NIP) — np. osoba założona wcześniej w aplikacji webowej:

  ```json
  409 { "error": { "code": "conflict", "message": "student with the same natural key already exists", "id": 4812 } }
  ```

  Dla firmy komunikat to `company with the same natural key already exists`. Nic nie zostało utworzone ani zmienione.
  `error.id` to istniejący rekord — jeśli to ta sama osoba/firma, zapamiętaj to `id` po swojej stronie i używaj go
  dalej (`GET`/`PATCH /students/{id}`, `studentId` w zaświadczeniu). API nie przypisuje `externalId` do istniejącego
  rekordu, więc kolejne `PUT` z tym `externalId` znów zwróci 409 — nie ponawiaj go w pętli.

### 6.3. Wystawienie zaświadczenia

Zalecany sposób dla integracji: **bez numeru rejestru i z kluczem idempotencji**.

```http
GET  /api/v1/courses/{courseId}                                # courses:read    — dostępne tłumaczenia
POST /api/v1/certificates                                      # certificates:write
Idempotency-Key: enrollment-8812
{
  "studentId": 15,
  "courseId": 128,
  "certificateDate": "2026-09-14",
  "courseDateStart": "2026-09-10",
  "courseDateEnd": "2026-09-12",
  "languageCode": "pl"
}
→ 201 { "data": { "id": 9812, "registryYear": 2026, "registryNumber": 43, "verificationCode": "K7QM4XPA9TZC" } }
GET  /api/v1/certificates/9812                                 # certificates:read
GET  /api/v1/certificates/9812/pdf                             # plik PDF
```

Numer rejestru:

- **Pominięty `registryNumber`** — serwer nadaje kolejny numer w obrębie (kurs, rok): największy istniejący + 1.
  Nadanie jest atomowe: równoległe żądania w tym samym kursie i roku dostają różne, kolejne numery, bez 409.
  Rok: `registryYear`, jeśli podany; inaczej rok `courseDateEnd`, a gdy jej brak — rok `courseDateStart`.
  Nadany rok i numer są w odpowiedzi.
- **Podany `registryNumber`** — zachowanie jak dotychczas: wymaga `registryYear`, musi być wolny (inaczej 409),
  numer bez roku → 400 `invalid certificate data`. Kolejny wolny numer podpowiada
  `GET /registries/next-number?courseId={courseId}&year=2026` (`registries:read`) — to podpowiedź, nie rezerwacja.

Szkolenie zdalne (e-learning): `courseDateStart` = dzień rozpoczęcia nauki, `courseDateEnd` = `certificateDate` =
dzień zaliczenia. API nie ma dla tego trybu osobnych reguł.

Idempotencja (`Idempotency-Key`, opcjonalny):

- Nieznany klucz → zaświadczenie powstaje, a klucz zapisywany jest w tej samej transakcji (`201`).
- Znany klucz i to samo ciało → nic nie powstaje, `200` z **tym samym ciałem** co pierwotne `201`.
  Dotyczy też żądań równoległych: drugie czeka na zakończenie pierwszego.
- Znany klucz i inne ciało → `409 idempotency key reused with different payload`, nic nie powstaje.
- „To samo ciało" = te same wartości pól; kolejność pól i białe znaki JSON nie mają znaczenia, pominięty
  `languageCode` to `pl`. Pominięty `registryNumber` i jawnie podany to **różne** ciała.
- Klucz: jeden nagłówek, 1–255 widocznych znaków ASCII bez spacji, inaczej 400 `invalid Idempotency-Key header`.
  Użyj identyfikatora zdarzenia po swojej stronie (np. id zapisu na kurs) — **ten sam przy każdym ponowieniu**.
- Klucz jest pamiętany co najmniej **30 dni**. Później może zostać usunięty; ponowienie po tym czasie wystawi
  nowe zaświadczenie.
- Żądanie zakończone błędem (400, 404, 409 numeru, 500) nie zapisuje klucza — poprawione żądanie możesz
  wysłać z tym samym kluczem.

Reguły walidacji:

- `certificateDate` ≥ `courseDateEnd` (albo `courseDateStart`, gdy brak końca), inaczej 400
  `certificate date cannot be before course end date`.
- `courseDateEnd` ≥ `courseDateStart`, inaczej 400 `invalid certificate data`.
- **Chronologia rejestru**: data zaświadczenia musi mieścić się między datami zaświadczeń o sąsiednich
  numerach w tym kursie i roku. Nadanie numeru 43 z datą wcześniejszą niż zaświadczenie nr 42 → 400
  `invalid certificate data`. Najbezpieczniej: zawsze kolejny numer i data nie wcześniejsza niż ostatnie zaświadczenie.
- Jawnie podany numer zajęty → 409. Przy 409 pobierz numer ponownie i powtórz z ograniczoną liczbą prób —
  albo pomiń numer i pozwól go nadać serwerowi. Chronologia rejestru obowiązuje także przy numerze nadanym
  przez serwer: data wcześniejsza niż data ostatniego zaświadczenia w kursie i roku → 400.
- `languageCode` ≠ `pl` wymaga tłumaczenia kursu w tym języku, inaczej 400 `certificate translation not found`.
- Nieistniejący kursant lub kurs → 404 `student not found` / `course not found`.

### 6.4. Dziennik szkolenia od utworzenia do zaświadczeń

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

### 6.5. Wygasające zaświadczenia

```http
GET /api/v1/dashboard                               # dashboard:read — do 50 najbliższych w ciągu 30 dni
GET /api/v1/certificates?dateFrom=2021-01-01&dateTo=2021-12-31&limit=100   # certificates:read
```

Pole `expiryDate` jest wyliczane (`courseDateEnd` + lata ważności × 365 dni) i nie da się po nim filtrować w API.

### 6.6. Synchronizacja katalogu kursów

Katalog platformy to kursy z `deliveredByPlatform = true`. Flagę ustawia CoursesList (strona kursu w aplikacji
webowej lub `PUT /courses/{id}/platform-delivery` z `courses:write`), nie platforma.

Pierwsze pobranie — wszystkie strony:

```http
GET /api/v1/courses/details?deliveredByPlatform=true&limit=100&page=1     # courses:read
→ 200 { "data": [ ...CourseDetails... ], "pagination": { "page": 1, "limit": 100, "total": 240, "totalPages": 3 } }
GET /api/v1/courses/details?deliveredByPlatform=true&limit=100&page=2
...
```

Kolejne pobrania — tylko zmiany od poprzedniej synchronizacji:

```http
GET /api/v1/courses/details?updatedSince=2026-09-15T08:00:00Z&deliveredByPlatform=true&limit=100&page=1
GET /api/v1/courses/details?updatedSince=2026-09-15T08:00:00Z&deliveredByPlatform=false&limit=100&page=1
```

- Pierwsze zapytanie zwraca kursy nowe lub zmienione w katalogu — nadpisz je u siebie.
- Drugie zwraca kursy zmienione i **nie** dostarczane przez platformę — w tym te, którym właśnie wyłączono flagę.
  Usuń je (ukryj) u siebie, jeśli je masz. Zmiana flagi liczy się jako zmiana kursu.
- `updatedSince` jest ściśle „po". Jako wartość weź moment rozpoczęcia poprzedniej udanej synchronizacji
  z zapasem (np. nagłówek `Date` pierwszej odpowiedzi minus 5 minut) i deduplikuj po `id` — zmiana zapisana
  w trakcie poprzedniego pobierania nie zginie.
- Zmianą kursu jest zmiana dowolnego pola, programu, dodanie/zmiana/usunięcie tłumaczenia i zmiana flagi.
  Zapis bez zmiany treści znacznika nie podnosi. Po wdrożeniu wszystkie kursy mają znacznik z chwili migracji.
- Stronicuj do `page = totalPages`. Kolejność jest stabilna, ale kurs zmieniony w trakcie stronicowania może się
  przesunąć między stronami — kolejna synchronizacja przyrostowa go dociągnie.
- API nie usuwa kursów, więc nie ma zdarzenia „kurs usunięty".

### 6.7. Webhooki — zdarzenia wychodzące

CoursesList sam wysyła zdarzenia do odbiorcy (platformy), także o zmianach wykonanych ręcznie w aplikacji webowej
(np. unieważnienie przez pracownika). Pełne schematy ciał: sekcja `webhooks` w `openapi.yaml`.

**Konfiguracja (po stronie CoursesList, poza API).** Odbiorcę dodaje administrator bazy:

```sql
INSERT INTO webhook_endpoints (name, url, secret)
VALUES ('platforma', 'https://platforma.example.pl/hooks/courseslist', '<wspólny sekret>');
-- wyłączenie: UPDATE webhook_endpoints SET active = false WHERE name = 'platforma';
```

Sekret ustalacie poza API. Zmienne środowiskowe API: `PUBLIC_BASE_URL` (publiczny adres API, z którego powstaje
`pdf_url`; bez niej pole jest pomijane) i `WEBHOOKS_ENABLED` (domyślnie `true`; przy kilku instancjach API można
wysyłać z dowolnej liczby — doręczenia nie dublują się). Zdarzenia trafiają tylko do odbiorców aktywnych w chwili
zapisu zmiany; nowy odbiorca nie dostaje zdarzeń wstecz.

**Transport.** `POST` na `url`, `Content-Type: application/json`, nagłówek
`X-Az-Signature: <HMAC-SHA256(sekret, surowe ciało), hex, małe litery>`. Licz HMAC z bajtów, które przyszły
(nie z ponownie zserializowanego obiektu), i porównuj stałoczasowo:

```go
mac := hmac.New(sha256.New, []byte(secret))
mac.Write(rawBody)
ok := hmac.Equal([]byte(r.Header.Get("X-Az-Signature")), []byte(hex.EncodeToString(mac.Sum(nil))))
```

**Zdarzenia.** Pola w `snake_case` — celowo inaczej niż `camelCase` w REST API.

| `event` | Kiedy | Pola |
|---|---|---|
| `certificate.issued` | `POST /certificates` z `Idempotency-Key` | `timestamp`, `idempotency_key`, `certificate_number`, `issued_at`; opcjonalnie `valid_until`, `pdf_url`, `verification_code` |
| `certificate.duplicate_issued` | wystawienie wtórnika (API lub aplikacja webowa) | `timestamp`, `certificate_number`, `duplicate_issued_at`, `reason` |
| `certificate.revoked` | unieważnienie (API lub aplikacja webowa) | `timestamp`, `certificate_number`, `reason` |
| `certificate.validity_changed` | `PATCH /certificates/{id}` zmienił termin ważności | `timestamp`, `certificate_number`, `valid_until` (`null` = bez terminu) |
| `program.updated` | zmiana nazwy, programu lub okresu ważności kursu z `deliveredByPlatform` | `timestamp`, `external_program_id` (= `id` kursu) |

```json
{"event":"certificate.issued","timestamp":"2026-09-15T10:15:00.123456Z","idempotency_key":"enrollment-8812",
 "certificate_number":"12/BHP/2026","issued_at":"2026-09-15","valid_until":"2031-09-14",
 "pdf_url":"https://courseslist.example.pl/api/v1/certificates/9812/pdf","verification_code":"K7QM4XPA9TZC"}
```

Wystawienie wtórnika tego samego zaświadczenia:

```json
{"event":"certificate.duplicate_issued","timestamp":"2026-10-02T08:41:12.004918Z",
 "certificate_number":"12/BHP/2026","duplicate_issued_at":"2026-10-02","reason":"Kursant zgubił oryginał"}
```

- **Zdarzenia o zaświadczeniach dotyczą wyłącznie dokumentów wystawionych z `Idempotency-Key`**. Dokumenty
  z aplikacji webowej i dzienników nie generują zdarzeń — odbiorca nie miałby ich z czym powiązać.
- **Duplikat nie jest nowym dokumentem.** `certificate.duplicate_issued` mówi tylko, że dla zaświadczenia
  o podanym numerze wydano wtórnik: numer rejestru, kod weryfikacyjny, data wystawienia i ważność zostają
  bez zmian, więc **nie twórz drugiego dokumentu** — odnotuj datę. Duplikat bywa wystawiany w aplikacji
  webowej (kursant zgubił oryginał), więc zdarzenie przychodzi też bez udziału platformy. Ponowne wystawienie
  wtórnika wysyła kolejne zdarzenie z nowszą datą.
- `pdf_url` wymaga klucza API z `certificates:read`; dla unieważnionego dokumentu zwraca 409.
- `timestamp` to ISO 8601 w UTC z `Z` i mikrosekundami. Rośnie ściśle w obrębie zaświadczenia (i kursu).

**Doręczanie.** Zdarzenie jest zapisywane w tej samej transakcji co zmiana i wysyłane dopiero po jej zatwierdzeniu
— nieudany zapis nie wysyła niczego, a niedostępny odbiorca nie blokuje wystawienia zaświadczenia.

| Odpowiedź odbiorcy | Zachowanie |
|---|---|
| 2xx | doręczone |
| brak odpowiedzi, timeout (10 s), 5xx | ponowienie po 1 min, 5 min, 15 min, 30 min, 1 h, 2 h, 4 h — łącznie 8 prób w ok. 8 h, potem `failed` |
| 400, 401, 422 i każdy inny kod (także 3xx, 404) | **bez ponowień** — `failed`, wpis `WEBHOOK ALERT` w logu API |

- Ponowienie wysyła **identyczne bajty** (ten sam `timestamp` i podpis).
- Kolejność: zdarzenia o jednym zaświadczeniu (i jednym kursie) są doręczane do danego odbiorcy w kolejności
  powstania — następne czeka, dopóki poprzednie jest ponawiane. Zdarzenie, które skończyło jako `failed`, nie
  blokuje kolejnych.
- Doręczenie jest „co najmniej raz": gdy odbiorca przetworzy zdarzenie, ale odpowiedź nie dotrze (timeout),
  przyjdzie ponownie. Traktuj powtórzony `timestamp` jako już zastosowany (odpowiedz 200).

Monitorowanie i ręczne ponowienie (operator):

```sql
SELECT d.id, ev.event_type, ev.subject_key, d.attempts, d.last_status_code, d.last_error, ev.payload::text
FROM webhook_deliveries d JOIN webhook_events ev ON ev.id = d.event_id
WHERE d.status = 'failed' ORDER BY d.id DESC;

-- po usunięciu przyczyny:
UPDATE webhook_deliveries SET status = 'pending', attempts = 0, next_attempt_at = now() WHERE id = <id>;
```

### 6.8. Publiczna weryfikacja zaświadczenia

Na wydruku PDF jest kod QR prowadzący pod adres `CERTIFICATE_VERIFICATION_URL` z podstawionym
`verificationCode` (domyślnie `/verify/{kod}` w aplikacji webowej CoursesList). Kryje się za nim
jedna trasa API:

```
GET /api/v1/public/certificates/{code}
```

- **Bez uwierzytelniania** — żadnego klucza API ani sesji; to jedyna taka trasa poza `/healthz`.
- Zwraca wyłącznie to, co i tak widnieje na papierze: `studentName` (imię, drugie imię, nazwisko),
  `certificateNumber`, `courseName`, `courseDateStart`, `courseDateEnd`, `issuedAt`, `validUntil`,
  `status` (`valid` albo `revoked`), `expired`, `duplicateIssued`, `duplicateIssuedAt`, `revokedAt`.
  **Bez PESEL, bez daty i miejsca urodzenia, bez firmy i bez powodu unieważnienia** — powód bywa
  wewnętrzną notatką.
- Kod jest normalizowany do wielkich liter. Zły format to `400 invalid verification code`,
  kod nieznany albo dokument usunięty to `404 certificate not found`.
- Ruch jest ograniczany **na kod**, nie na adres IP (przeglądarka woła API przez proxy aplikacji
  webowej, więc wszystkie żądania mają ten sam adres): po przekroczeniu limitu `429 too many requests`.
- Odpowiedź ma `Cache-Control: no-store` i `X-Robots-Tag: noindex`.

Integracja serwer-serwer nie potrzebuje tej trasy — ma `GET /certificates/by-verification-code/{code}`
z pełnymi danymi i zakresem `certificates:read` (sekcja 5). Trasa publiczna istnieje dla osoby,
która trzyma wydruk w ręce.

Gdy `CERTIFICATE_VERIFICATION_URL` nie jest ustawiony, kod QR nie jest drukowany, a `CertificateDetails`
nie zawiera `verificationUrl` ani `verificationQr`. Sama trasa publiczna działa niezależnie od tej zmiennej.

---

### 6.9. Nadruki na wydruku platformowym

Zaświadczenie wystawione z `Idempotency-Key` trafia do kursanta elektronicznie — nikt nie
przystawia na nim pieczątki ręcznie, jak na wydruku z aplikacji webowej. Taki dokument dostaje
więc na PDF **trzy pieczątki, podpis i giloszowe tło** na obu stronach. Każdy z nadruków
jest opcjonalny - dopóki administrator nie wgra pliku, nie pojawia się nic.

Kryterium jest to samo, po którym rozpoznajesz dokumenty platformy przy webhookach:
niepusty klucz idempotencji. Dokument wystawiony bez tego nagłówka drukuje się dokładnie
tak jak dotąd.

Umiejscowienie wybiera szablon kursu znacznikami `{{ pieczatka_okragla }}`,
`{{ pieczatka_firmowa }}`, `{{ pieczatka_imienna }}` i `{{ podpis }}` - nazwy mówią, która
pieczątka trafia w dane miejsce. Szablon bez znaczników dostaje pasek u dołu pierwszej
strony, obok kodu QR; nadruki są tam skalowane tak, żeby komplet zmieścił się w wierszu.
Znacznik bez wgranego pliku znika z wydruku bez śladu.

Pliki wgrywa administrator CoursesList w przeglądarce (`POST /admin/certificate-print-assets/{kind}`);
klucz API tej trasy nie otworzy — dostanie `403 this endpoint requires an interactive session`.
Integracja może je tylko odczytać: `GET /certificate-print-assets` (metadane) oraz
`GET /certificate-print-assets/{kind}/file` i `/certificate-print-assets/guilloche` (obrazy).

Szczegóły zaświadczenia niosą `printDecor` z adresami nadruków — pole jest obecne **wyłącznie**
przy dokumentach platformowych, więc integracja nie musi powtarzać kryterium.

Tło giloszowe jest wkompilowane w API i nie podlega konfiguracji; pochodzi z blankietu
organizatora (`docs/api/wzory`).

---

---

## 7. Obsługa błędów — zalecenia

| Status | Ponawiać? | Uwagi |
|---|---|---|
| 400 | Nie | Błąd po stronie aplikacji. Ogólne `invalid request body` nie wskazuje pola (brak wymaganego pola, pole spoza schematu, zła data) — sprawdź sekcje 4 i 5 oraz schemat; walidacja NIP-u, programu kursu i tłumaczeń zwraca konkretne komunikaty. |
| 401 | Nie | Klucz nieważny — zatrzymaj integrację i zgłoś potrzebę nowego klucza. |
| 403 | Nie | Brak zakresu lub uprawnień administratora. |
| 404 | Nie | — |
| 409 | Zależy | Konflikt stanu. Dla jawnie podanego numeru rejestru: pobierz nowy numer i ponów. `certificate already revoked` przy ponowieniu unieważnienia oznacza, że pierwsze wywołanie się powiodło — pobierz dokument przez GET. `idempotency key reused with different payload` — nie ponawiaj, to błąd po stronie klienta (ten sam klucz dla różnych zaświadczeń). |
| 429 | Tak, z opóźnieniem | Tylko logowanie i publiczna weryfikacja (sekcja 6.8) — trasy z kluczem API nie mają limitu. Odczekaj minutę. |
| 500 | Ostrożnie | Błąd serwera. `POST /certificates` z `Idempotency-Key` możesz bezpiecznie ponawiać z opóźnieniem. Pozostałe operacje `POST` nie są idempotentne — ponów najwyżej raz. |
| Brak odpowiedzi / timeout | Ostrożnie | `POST` mógł zostać wykonany. `POST /certificates` z `Idempotency-Key` ponów z tym samym kluczem i ciałem (dostaniesz `200` z pierwotnym wynikiem, jeśli dokument powstał). Dla pozostałych operacji przed ponowieniem sprawdź, czy obiekt nie powstał. |

Klucze idempotencji obsługuje wyłącznie `POST /certificates` (sekcja 6.3). Operacje
`PUT .../by-external-id/{externalId}` są idempotentne same z siebie (sekcja 6.2). Pozostałe operacje zapisu ich nie
obsługują — nagłówek `Idempotency-Key` jest przez nie ignorowany.

---

## 8. Czego API nie oferuje

- zdarzeń webhook o zaświadczeniach wystawionych bez `Idempotency-Key` (aplikacja webowa, dzienniki), o zmianach
  kursantów i firm ani o zmianach kursów spoza katalogu platformy — te zmiany trzeba odpytywać;
- zarządzania odbiorcami webhooków przez API (konfiguracja w bazie, sekcja 6.7) ani ponownego wysłania zdarzenia
  na żądanie;
- filtrowania „zmienione od" (`updatedSince`) i paginacji dla list innych niż kursy (kursanci, firmy,
  zaświadczenia, dzienniki są ograniczone do 100 wyników na zapytanie);
- przypisania `externalId` do istniejącego kursanta lub firmy — po 409 z `error.id` powiązanie trzymasz po swojej
  stronie (sekcja 6.2);
- cofnięcia unieważnienia zaświadczenia;
- wydruku PDF unieważnionego zaświadczenia;
- zmiany pieczątek ani podpisu przez API - wgrywa je administrator w przeglądarce (sekcja 6.9),
  a tło giloszowe jest wkompilowane w API i nie ma ustawienia, które by je wyłączało;
- sterowania kodem QR na wydruku — pojawia się na każdym dokumencie, gdy adres weryfikacji jest
  skonfigurowany; jego miejsce wybiera się wyłącznie znacznikiem `{{ kod_qr }}` w szablonie kursu
  (bez znacznika ląduje w prawym dolnym rogu);
- idempotencji operacji zapisu innych niż `POST /certificates` (poza naturalnie idempotentnymi
  `PUT .../by-external-id/...`, unieważnieniem i duplikatem);
- operacji zbiorczych (np. obecność wielu osób jednym żądaniem);
- usuwania kursantów, firm i kursów;
- wersjonowania poza prefiksem `/api/v1` — specyfikacja opisuje stan kodu z gałęzi `api_and_webhook`
  i nie jest gwarancją zgodności na przyszłość.

Jeśli integracja potrzebuje którejś z tych rzeczy, zgłoś to właścicielowi API zamiast obchodzić ograniczenie.
