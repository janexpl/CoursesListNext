# Zlecenie: powierzchnia integracyjna API CoursesList

Dokument dla agenta pracującego w repozytorium **CoursesList** (Go API, gałąź
`api_and_webhook`). Opisuje, co dołożyć do `/api/v1`, żeby zewnętrzna platforma
e-learningowa mogła wydawać zaświadczenia za kursy ukończone zdalnie.

Czytaj razem z `INTEGRATION.md` i `openapi.yaml` w tym repozytorium — one
opisują stan dzisiejszy, ten plik opisuje różnicę.

---

## 0. Kontekst: kto to woła i po co

Platforma e-learningowa prowadzi kursy BHP w całości zdalnie: kursant przechodzi
materiał, zdaje egzamin, po czym **musi dostać zaświadczenie z CoursesList**.
CoursesList pozostaje jedynym źródłem prawdy dla zaświadczeń — platforma nigdy
nie nadaje numerów rejestru, nie liczy dat ważności ani nie generuje dokumentu.

Ruch jest jednokierunkowy: platforma woła API, CoursesList odpowiada, a zmiany
stanu zaświadczeń wracają webhookiem. Platforma nie ma dostępu do bazy.

Skala dzisiaj jest mała — kilkanaście kursów, pojedynczy kursanci. Nie projektuj
pod tysiące żądań na minutę. Projektuj pod **poprawność przy ponowieniach**,
bo to jest jedyne miejsce, w którym błąd oznacza dwa zaświadczenia dla jednej
osoby albo brak zaświadczenia po opłaconym szkoleniu.

---

## 1. Zasada naczelna: dokładaj, nie przerabiaj

Aplikacja webowa i dotychczasowe API działają. **Żadna zmiana z tego zlecenia
nie może zmienić zachowania istniejących operacji dla dotychczasowych klientów.**

Konkretnie:

- nowe pola w odpowiedziach są dopuszczalne (klienci je zignorują),
- nowe pola w żądaniach muszą być **opcjonalne** — pamiętaj, że schematy `*Write`
  mają `additionalProperties: false`, więc dodanie pola wymaga zmiany schematu,
  a nie tylko kodu,
- pole dziś wymagane nie staje się opcjonalne bez wyraźnego punktu niżej,
- nie zmieniaj kształtu `CourseDetails` — druga strona już go parsuje i zgadza
  się co do pola.

Każdą zmianę odzwierciedl w `openapi.yaml` (z `x-required-scope`) i w
`INTEGRATION.md`. Specyfikacja jest tu kontraktem, nie dokumentacją po fakcie.

---

## 2. Rozstrzygnięcie, które zapada tutaj i nie podlega negocjacji w kodzie

Kontrakt po stronie platformy zakładał pierwotnie, że wydanie zaświadczenia
polega na wysłaniu **migawki danych** (imię, nazwisko, data i miejsce urodzenia,
program kursu, liczba godzin) i że CoursesList sam utworzy z tego dokument.

**Odrzucamy to.** Zaświadczenie w CoursesList ma `studentId` i `courseId`,
a listy `GET /students/{id}/certificates` i `GET /courses/{id}/certificates` na
tym stoją. Dokument bez kursanta byłby sierotą niewidoczną w aplikacji webowej
— czyli dokładnie tym, czego się nie chce w ewidencji.

Obowiązuje więc przepływ:

1. platforma zakłada (albo odnajduje) **firmę** — punkt 6,
2. platforma zakłada (albo odnajduje) **kursanta** — punkt 6,
3. platforma woła wydanie zaświadczenia, podając `studentId` i `courseId`
   — punkt 3.

`courseId` platforma już zna: przechowuje `id` kursu z CoursesList od czasu
synchronizacji katalogu. Nie trzeba niczego mapować po nazwie.

Konsekwencja, którą trzeba znać: **platforma musi mieć datę i miejsce urodzenia
kursanta, zanim wyda zaświadczenie.** To jej problem, nie Twój — nie osłabiaj
wymagalności `birthDate` ani `birthPlace` w `StudentWrite`, bo te dane trafiają
na dokument.

---

## 3. Wydanie zaświadczenia: idempotencja i numer nadawany po stronie serwera

To jest najważniejszy punkt całego zlecenia. Dziś `POST /certificates` wymaga,
żeby **klient sam wybrał `registryNumber`**, a `GET /registries/next-number` jest
— zgodnie z własnym opisem — „podpowiedzią, nie rezerwacją". Do tego dochodzi
reguła chronologii (data zaświadczenia musi mieścić się między datami sąsiednich
numerów) i brak kluczy idempotencji przy nieidempotentnym `POST`.

Dla integracji maszynowej to jest niewykonalne bezpiecznie. Wywołanie, które
zakończy się timeoutem, zostawia platformę bez wiedzy, czy dokument powstał;
ponowienie z tym samym numerem da 409, a z kolejnym — **drugie zaświadczenie
dla tej samej osoby za ten sam kurs**.

### 3.1 Nagłówek `Idempotency-Key`

`POST /certificates` przyjmuje opcjonalny nagłówek:

```http
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
```

Zachowanie:

- klucz nieznany → normalne utworzenie; zapamiętaj klucz razem z `id` powstałego
  zaświadczenia **w tej samej transakcji, co wstawienie zaświadczenia**;
- klucz znany → **nie twórz nic**, zwróć `200` z tym samym ciałem, co pierwotne
  `201`. Nie `409` — powtórzenie jest normalnym zachowaniem klienta po timeoucie,
  a nie błędem;
- klucz znany, ale ciało żądania **różni się** od pierwotnego → `409`
  `idempotency key reused with different payload`. To sygnał błędu po stronie
  klienta i nie wolno go zamaskować zwróceniem starego dokumentu.

Klucz jest unikalny globalnie, nie per kurs. Trzymaj go w osobnej tabeli
(`idempotency_keys`: klucz, odcisk ciała żądania, id zaświadczenia, znacznik
czasu), z ograniczeniem unikalności na kluczu — to ograniczenie jest tu realnym
mechanizmem, nie ozdobą. Wpisy starsze niż 30 dni możesz kasować.

Platforma wysyła jako klucz UUID jednego podejścia egzaminacyjnego. Ponawia do
5 razy z narastającym odstępem (60 s, ×2), przez pół godziny. Brak odpowiedzi
2xx oznacza **identyczne** żądanie za chwilę, nie nowe.

### 3.2 Numer rejestru nadawany przez serwer

`registryNumber` i `registryYear` stają się **opcjonalne**:

- podane — zachowanie dokładnie jak dziś, z dzisiejszą walidacją i 409;
- pominięte — serwer nadaje sam: rok z `courseDateEnd` (a gdy go brak, z
  `courseDateStart`), numer = największy istniejący w tym (kurs, rok) + 1.

Nadanie musi być **atomowe wobec współbieżnych wydań w tym samym kursie i roku**.
Blokada na wierszu kursu albo ograniczenie unikalności `(courseId, registryYear,
registryNumber)` z ponowieniem w pętli — nie odczyt „ostatniego numeru" poza
transakcją. Tu jeden błąd oznacza dwa dokumenty z tym samym numerem w rejestrze.

Ponieważ numer jest zawsze kolejny, a data nie jest wcześniejsza niż ostatnie
zaświadczenie, reguła chronologii spełnia się sama. Gdyby jednak data łamała
chronologię (zaświadczenie wydawane wstecz), zwróć dotychczasowy błąd 400 —
nie próbuj wciskać dokumentu w środek rejestru.

### 3.3 Daty dla szkolenia zdalnego

Platforma nie ma „edycji kursu" z datami. Będzie wysyłać `courseDateStart` =
dzień rozpoczęcia nauki, `courseDateEnd` = `certificateDate` = dzień zaliczenia.
Dzisiejsze reguły (`certificateDate ≥ courseDateEnd`, `courseDateEnd ≥
courseDateStart`) są wtedy spełnione i **nie wymagają żadnej zmiany**. Wspominam
o tym tylko po to, żebyś nie dokładał walidacji zakładającej wielodniowy kurs.

---

## 4. Kod weryfikacyjny

Platforma wystawia publiczną stronę „sprawdź zaświadczenie" pod adresem
zawierającym kod. Dziś API nie zwraca żadnej takiej wartości, więc ta funkcja po
prostu nie działa.

Dołóż do zaświadczenia pole `verificationCode`:

- generowane przy tworzeniu dokumentu, **także z aplikacji webowej i z dziennika**
  — nie tylko przy wywołaniu przez API, inaczej powstaną dokumenty bez kodu;
- losowe, z generatora kryptograficznego, nie sekwencyjne i niewyprowadzalne
  z numeru rejestru (inaczej da się zgadywać cudze zaświadczenia);
- krótkie i czytelne dla człowieka przepisującego je z papieru: 10–12 znaków
  z alfabetu bez znaków mylących (bez `0`, `O`, `1`, `I`, `l`);
- unikalne w całej tabeli, z ograniczeniem unikalności w bazie;
- niezmienne przez całe życie dokumentu.

Zwracaj je w `CertificateDetails` i w odpowiedzi na utworzenie. Istniejące
zaświadczenia uzupełnij migracją.

Rozważ też `GET /certificates/by-verification-code/{code}` (zakres
`certificates:read`) zwracające ten sam kształt co `GET /certificates/{id}` —
przydatne, jeśli weryfikacja ma kiedyś działać także po stronie CoursesList.
To jest opcjonalne; platforma trzyma kod u siebie i bez tego sobie poradzi.

---

## 5. Unieważnienie i duplikat

Dziś jedyną operacją zdejmującą dokument jest `DELETE /certificates/{id}`, która
**kasuje go trwale i zwalnia numer rejestru do ponownego użycia**. Dla dokumentu
formalnego to nie jest unieważnienie — ślad znika, a numer może zostać nadany
komuś innemu.

### 5.1 `POST /certificates/{id}/revoke`

Zakres `certificates:write`. Ciało:

```json
{ "reason": "Błędne dane kursanta" }
```

`reason` wymagany, niepusty. Efekt: dokument zostaje w bazie, dostaje
`revokedAt` i `revokeReason`, a jego **numer rejestru pozostaje zajęty na zawsze**.
Unieważniony dokument:

- dalej jest widoczny w `GET /certificates/{id}` i na listach, z widocznym stanem,
- **nie może** zostać unieważniony ponownie → 409 `certificate already revoked`,
- PDF: zdecyduj świadomie i zapisz decyzję w `INTEGRATION.md` — albo 409, albo
  dokument ze znakiem unieważnienia. Nie zostawiaj tego przypadkowi.

### 5.2 `POST /certificates/{id}/duplicate`

Zakres `certificates:write`. Ciało jak wyżej. Tworzy **nowy** dokument z danymi
skopiowanymi z oryginału, z nowym numerem rejestru nadanym jak w punkcie 3.2,
i wiąże go z oryginałem polem `supersedesId`. Oryginał dostaje wskazanie, że
został zastąpiony. Odpowiedź `201` z pełnym nowym dokumentem.

Obie operacje wysyłają webhook (punkt 7).

---

## 6. Znajdź-lub-utwórz kursanta i firmę po identyfikatorze platformy

Platforma zakłada kursanta w momencie, w którym ten dostaje dostęp do szkolenia,
i musi móc powtórzyć to wywołanie bez tworzenia duplikatu. Dziś się nie da:
nie ma find-or-create, a klucz naturalny (imię, nazwisko, data urodzenia) daje
409 przy powtórce, którego nie sposób odróżnić od realnego konfliktu z inną osobą.

### 6.1 Pole `externalId`

Dołóż do kursanta i do firmy nullowalne pole `externalId` (string, maks. 64
znaki) z **unikalnością częściową** — unikalny, gdy niepusty; wiele `null`
dopuszczalnych, bo rekordy zakładane w aplikacji webowej go nie mają.

Platforma wpisuje tam własny UUID. Nigdy go nie interpretuj ani nie parsuj.

### 6.2 `PUT /students/by-external-id/{externalId}` i `PUT /companies/by-external-id/{externalId}`

Zakresy odpowiednio `students:write` i `companies:write`. Ciało: istniejące
`StudentWrite` / `CompanyWrite` **bez** pola `externalId` (jest w ścieżce).

Semantyka:

- brak rekordu o tym `externalId` → utwórz, `201`;
- rekord istnieje → zaktualizuj i zwróć `200`.

Świadomie `PUT`, nie `PATCH`: pełne nadpisanie jest tu zamierzone i zgodne
z regułą z sekcji 4 `INTEGRATION.md`, a nazwa metody wreszcie mówi prawdę
o zachowaniu.

Pułapka do rozstrzygnięcia jawnie: **co zrobić, gdy dane w ciele kolidują z
kluczem naturalnym innego rekordu** (istnieje już inny kursant o tym imieniu,
nazwisku i dacie urodzenia, ale z innym albo pustym `externalId`). Nie twórz
wtedy duplikatu i nie nadpisuj cudzego rekordu. Zwróć `409` z komunikatem
`student with the same natural key already exists` **i dołóż w ciele błędu `id`
kolidującego rekordu** — platforma musi móc zdecydować, czy podpiąć się pod
istniejącą osobę. Bez tego id integracja utyka i wymaga ręcznej interwencji.

### 6.3 `telephone` firmy przestaje być wymagany

`CompanyWrite.telephone` jest dziś wymagany, a platforma tego numeru nie ma
i nie ma skąd wziąć — GUS go nie zwraca, co `INTEGRATION.md` sam odnotowuje.
Zmień na opcjonalny (nullowalny). Numer telefonu firmy nie trafia na
zaświadczenie, więc nic to nie psuje, a dziś blokuje założenie klienta B2B.

To jedyne poluzowanie wymagalności w całym zleceniu. Reszta zostaje.

---

## 7. Webhook wychodzący

Bez tego platforma nie dowie się o niczym, co wydarzy się w aplikacji webowej.
Unieważnienie wpisane ręcznie przez pracownika musi dotrzeć do platformy —
i to jest właściwy powód istnienia tego punktu, ważniejszy niż potwierdzanie
operacji wywołanych przez samą platformę.

### 7.1 Transport i podpis

`POST` na adres skonfigurowany per odbiorca, `Content-Type: application/json`.
Nagłówek:

```http
X-Az-Signature: <HMAC-SHA256 z SUROWEGO ciała żądania, hex, małe litery>
```

Sekret wspólny, ustalany poza API. Podpisuj **dokładnie te bajty, które wysyłasz**
— nie ponownie serializowany obiekt. Odbiorca liczy HMAC z surowego ciała
i porównuje stałoczasowo; każda zmiana formatowania po podpisaniu da 401.

### 7.2 Zdarzenia

| `event` | Pola wymagane | Pola opcjonalne |
|---|---|---|
| `certificate.issued` | `timestamp`, `idempotency_key`, `certificate_number`, `issued_at` | `valid_until`, `pdf_url`, `verification_code` |
| `certificate.revoked` | `timestamp`, `certificate_number`, `reason` | — |
| `certificate.validity_changed` | `timestamp`, `certificate_number`, `valid_until` | — |
| `program.updated` | `timestamp`, `external_program_id` | — |

Nazwy pól w ciele webhooka są w `snake_case` — **celowo inne niż `camelCase`
w REST API**. Odbiorca już je tak parsuje; nie „ujednolicaj" tego.

Przykład:

```json
{
  "event": "certificate.issued",
  "timestamp": "2026-09-15T10:15:00Z",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "certificate_number": "12/BHP/2026",
  "issued_at": "2026-09-15",
  "valid_until": "2029-09-15",
  "pdf_url": "https://courseslist.example.pl/api/v1/certificates/9812/pdf",
  "verification_code": "K7M4XP9RTQ"
}
```

`timestamp` w ISO 8601 z `Z` — **inaczej niż w REST API**, gdzie znaczniki czasu
są lokalne i bez strefy. Odbiorca odrzuca zdarzenia starsze niż ostatnio
zastosowane dla danego dokumentu, więc znacznik musi rosnąć per zaświadczenie.

`program.updated` wysyłaj przy każdej zmianie kursu wpływającej na treść
zaświadczenia — nazwa, program, okres ważności. `external_program_id` to `id`
kursu w CoursesList.

### 7.3 Doręczanie

Odbiorca odpowiada `200` (przetworzone albo świadomie pominięte jako przestarzałe),
`400` (brak wymaganego pola), `401` (zły podpis), `422` (nieznany dokument).

- ponawiaj wyłącznie przy **braku odpowiedzi, timeoucie i 5xx** — z narastającym
  odstępem, co najmniej 5 prób w ciągu kilku godzin;
- **nie ponawiaj przy 400, 401 i 422** — to błędy trwałe, ponawianie ich zapętla
  kolejkę. Zaloguj i zgłoś operatorowi;
- kolejkuj w bazie ze stanem, nie wysyłaj synchronicznie w transakcji zapisu.
  Zaświadczenie musi powstać nawet wtedy, gdy platforma nie odpowiada;
- wysyłkę wyzwalaj po zatwierdzeniu transakcji, inaczej wyślesz zdarzenie
  o dokumencie, który nie powstał.

---

## 8. Katalog kursów: filtr przyrostowy i flaga dostarczania

### 8.1 `updatedSince`

`GET /courses` i `GET /courses/details` przyjmują opcjonalny `updatedSince`
(ISO 8601). Zwracają kursy zmienione **po** tym momencie. Wymaga kolumny
`updated_at` aktualizowanej przy każdej zmianie kursu, wraz z tłumaczeniami
i programem — zmiana samego tłumaczenia też musi podnieść znacznik.

### 8.2 Paginacja

Obie listy przyjmują `page` (od 1) i zwracają kopertę z `pagination`, tak jak
`GET /companies/{id}/certificates` dziś. Bez tego katalog powyżej 100 kursów
jest nie do pobrania — `limit` bez `page` to ograniczenie, nie stronicowanie.

### 8.3 `deliveredByPlatform`

Dołóż do kursu flagę logiczną, domyślnie `false`, edytowalną w aplikacji webowej,
oraz parametr filtrujący na obu listach.

Powód jest po stronie platformy i jest twardy: CoursesList zawiera **całą** ofertę,
w tym szkolenia wymagające części praktycznej (wózki, suwnice, spawanie, SEP),
których nie da się zrealizować zdalnie. Bez tej flagi w katalogu platformy
pojawiają się kursy, których nie da się uruchomić, widoczne dla kursantów.

**Decyzja, który kurs jest dostarczany przez platformę, należy do CoursesList,
nie do platformy** — to ten system wie, co jest w ofercie i w jakiej formie.

---

## 9. Czego NIE robić

- **nie ruszaj modelu dziennika** (`journals`) — szkolenia zdalne nie mają sesji
  ani obecności, platforma nie będzie tych endpointów wołać;
- **nie dodawaj kasowania kursantów, firm ani kursów** — to nie jest potrzebne
  i nie jest przedmiotem tego zlecenia;
- **nie zmieniaj `DELETE /certificates/{id}`** — zostaje jak jest, dla
  administratora. Unieważnienie z punktu 5 to osobna operacja, nie jego zamiennik;
- **nie wprowadzaj `/api/v2`** — wszystko mieści się w zgodnych rozszerzeniach
  `/api/v1`;
- **nie zmieniaj kształtu `CourseDetails` ani semantyki `expiryTime`** (`null` =
  bezterminowy, `0` = ważne do końca kursu). Druga strona właśnie pod to pisze kod.

---

## 10. Kolejność pracy

Uszeregowana tym, co blokuje platformę najmocniej. Każdy punkt jest osobnym,
działającym krokiem — nie rób wszystkiego naraz.

1. **Punkt 3** — idempotencja i numer nadawany przez serwer. Bez tego nie da się
   bezpiecznie wydać ani jednego zaświadczenia maszynowo.
2. **Punkt 6** — `externalId` i znajdź-lub-utwórz. Bez tego nie da się dojść do
   punktu 3, bo nie ma `studentId`.
3. **Punkt 4** — kod weryfikacyjny.
4. **Punkt 8** — filtr przyrostowy, paginacja i flaga dostarczania.
5. **Punkt 5** — unieważnienie i duplikat.
6. **Punkt 7** — webhook. Ostatni, bo platforma ma awaryjne odpytywanie, ale
   docelowo wymagany: przy unieważnieniu dokumentu formalnego godzina opóźnienia
   bywa nieakceptowalna.

---

## 11. Warunki odbioru

Dla każdego punktu test **automatyczny**, nie ręczne sprawdzenie w kliencie HTTP.
Te przypadki są obowiązkowe, bo każdy z nich to błąd cichy i kosztowny:

- dwa `POST /certificates` z **tym samym** `Idempotency-Key` → jedno
  zaświadczenie w bazie, drugie wywołanie zwraca `200` z tym samym `id`;
- ten sam klucz z **innym** ciałem → `409`, bez utworzenia dokumentu;
- **równoległe** wydania w tym samym kursie i roku bez podanego numeru →
  numery kolejne i różne, zero duplikatów. Test musi puszczać żądania naprawdę
  współbieżnie, sekwencyjny niczego tu nie dowodzi;
- `PUT .../by-external-id/{id}` dwa razy → jeden rekord, drugie wywołanie `200`;
- to samo, gdy klucz naturalny koliduje z innym rekordem → `409` **z `id`
  kolidującego rekordu w ciele**;
- unieważnienie → numer rejestru pozostaje zajęty, ponowne unieważnienie `409`;
- duplikat → nowy numer, `supersedesId` wskazuje oryginał;
- `verificationCode` unikalny i nadawany także przy tworzeniu z aplikacji
  webowej oraz z dziennika, nie tylko przez API;
- webhook: podpis policzony z surowego ciała zgadza się po stronie odbiorcy;
  5xx powoduje ponowienie, a 400/401/422 **nie**;
- `updatedSince` zwraca kurs po zmianie samego tłumaczenia;
- istniejące testy przechodzą bez zmian. Jeśli któryś zaczyna padać, to jest
  sygnał, że zmiana nie jest wstecznie zgodna — zgłoś zamiast poprawiać test.

Na koniec zaktualizuj `openapi.yaml` i `INTEGRATION.md`, w tym sekcję 8
„Czego API nie oferuje", z której część pozycji przestaje być prawdą.
