# Środowisko deweloperskie

## Dlaczego API w kontenerze

Na macOS nie da się sensownie renderować zaświadczeń:

- markowy **Google Chrome** zapisuje PDF, ale nie kończy procesu (updater i crashpad
  żyją dalej), więc `pdfutil` czeka do 20-sekundowego timeoutu i zwraca błąd, mimo że
  plik powstał;
- **wkhtmltopdf** z Homebrew jest binarką x86 (`bad CPU type in executable` na Apple
  Silicon), a projekt jest archiwalny — produkcja i tak używa Chromium.

Doraźnie można zainstalować Chromium przez `brew install --cask chromium` i wskazać go
w `CHROME_BIN`, ale PDF-y będą się renderować macowymi fontami, czyli inaczej niż na
serwerze. `docker-compose.dev.yml` uruchamia API na obrazie z tym samym Chromium
i tymi samymi fontami Liberation co produkcja — to, co zobaczysz lokalnie, jest tym,
co dostanie kursant.

**Frontend i testy zostają na hoście.** Nuxt w Dockerze na macOS ma wolne wykrywanie
zmian w plikach, a `go test` na hoście startuje w sekundę.

## Start

### Wariant 1: baza na hoście (domyślny)

Wymaga jednorazowego dopuszczenia połączeń z Dockera do lokalnego Postgresa (niżej).

```bash
docker compose -f docker-compose.dev.yml up api     # API na http://localhost:8081
cd web && pnpm dev                                  # frontend na http://localhost:3000
```

### Wariant 2: baza w kontenerze (bez zmian w lokalnym Postgresie)

```bash
DEV_DB_HOST=db docker compose -f docker-compose.dev.yml --profile db up
```

Przy pierwszym starcie baza dostaje `api/internal/db/schema.sql`, czyli pusty schemat
zgodny z kodem — bez danych i bez użytkowników. Konto do logowania zakładasz sam,
np. kopiując wiersz z bazy na hoście albo wstawiając go ręcznie. Port na hoście to
**55432**, żeby nie kolidował z lokalnym Postgresem:

```bash
psql -h 127.0.0.1 -p 55432 -U janusz -d courselist
```

Dane bazy deweloperskiej żyją w wolumenie `dev-db-data`; kasujesz je przez
`docker compose -f docker-compose.dev.yml --profile db down -v`.

## Dopuszczenie Dockera do Postgresa na hoście (wariant 1)

Homebrew konfiguruje Postgresa tak, że słucha wyłącznie na `localhost`, więc kontener
go nie zobaczy. Dwie zmiany i restart:

```bash
PGDATA=/opt/homebrew/var/postgresql@15

# 1. Nasłuch także na adresie, pod którym host widzi Dockera.
echo "listen_addresses = '*'" >> $PGDATA/postgresql.conf

# 2. Dostęp z sieci Docker Desktop (sprawdź adres: docker run --rm alpine ip route | awk '/default/{print $3}').
echo "host    all    all    192.168.65.0/24    trust" >> $PGDATA/pg_hba.conf

brew services restart postgresql@15
```

Uwagi:

- `listen_addresses = '*'` oznacza, że Postgres nasłuchuje też na interfejsie Wi‑Fi.
  Na laptopie w obcej sieci warto mieć włączoną zaporę macOS albo wracać do
  `listen_addresses = 'localhost'`, gdy akurat nie używasz kontenera.
- `trust` jest tu spójne z tym, co już masz dla `127.0.0.1`. Jeśli wolisz hasło,
  ustaw je dla roli i dopisz `DB_PASS` do `api/.env`.
- Restart zrywa otwarte połączenia, także te z TablePlus — wystarczy połączyć się
  ponownie.

## Codzienna praca

| Sytuacja | Polecenie |
|---|---|
| zmiana kodu Go | `docker compose -f docker-compose.dev.yml restart api` |
| zmiana zależności Go, Dockerfile.dev | `docker compose -f docker-compose.dev.yml build api` |
| logi API | `docker compose -f docker-compose.dev.yml logs -f api` |
| shell w kontenerze | `docker compose -f docker-compose.dev.yml exec api bash` |
| zatrzymanie | `docker compose -f docker-compose.dev.yml down` |

Kod jest montowany z hosta, więc `go run` w kontenerze widzi Twoje zmiany od razu —
restart jest po to, żeby proces wystartował na nowo. Cache modułów i kompilacji siedzi
w wolumenach `dev-go-mod` i `dev-go-build`, więc restart trwa sekundy.

Testy uruchamiasz jak dotąd, na hoście:

```bash
cd api && go test ./...
TEST_DATABASE_URL='postgres://janusz@127.0.0.1:5432/postgres?sslmode=disable' go test ./internal/integrationtest/
```

## Sprawdzenie renderowania PDF

```bash
curl -s -o /tmp/zaswiadczenie.pdf -w '%{http_code} %{content_type}\n' \
  -H "Authorization: Bearer clk_..." \
  http://localhost:8081/api/v1/certificates/<id>/pdf
file /tmp/zaswiadczenie.pdf     # powinno być: PDF document
```

Gdy coś nie gra:

- `chrome executable not found` — obraz zbudował się bez Chromium; przebuduj
  (`build api`) albo sprawdź `CHROME_BIN` w kontenerze;
- `chrome pdf failed: ...` z treścią o timeoucie — zwykle znaczy, że API działa na
  hoście (macOS), a nie w kontenerze;
- pusta strona w PDF — szablon zaświadczenia ładuje zasób z sieci; render nie ma
  dostępu do internetu poza tym, co jest w obrazie.
