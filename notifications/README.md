# Certificate Expiry Notifications

Worker pobiera przez API listę zaświadczeń z kończącą się ważnością, grupuje je po firmie i wysyła jeden zbiorczy e-mail do klienta.

## Konfiguracja

- `NOTIFICATIONS_API_BASE_URL`: adres API, np. `http://api:8082`.
- `NOTIFICATIONS_API_TOKEN`: bearer token zgodny z API.
- `NOTIFICATIONS_LOOKAHEAD_DAYS`: zakres wyszukiwania od dzisiaj, domyślnie `30`.
- `NOTIFICATIONS_LIMIT`: rozmiar strony API, domyślnie `500`.
- `NOTIFICATIONS_STATE_FILE`: plik stanu wysłanych powiadomień, domyślnie `/data/notifications-state.json`.
- `NOTIFICATIONS_RUN_INTERVAL`: opcjonalny interwał pracy, np. `24h`; puste oznacza jednorazowe uruchomienie.
- `NOTIFICATIONS_DRY_RUN`: `true` nie wysyła e-maili i nie zapisuje stanu.
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_FROM_NAME`: konfiguracja SMTP.
- `SMTP_TLS_MODE`: `auto`, `starttls`, `tls` albo `none`; domyślnie `auto`.

## Uruchomienie lokalne

```bash
cd notifications
NOTIFICATIONS_API_BASE_URL=http://localhost:8081 \
NOTIFICATIONS_API_TOKEN=token \
NOTIFICATIONS_DRY_RUN=true \
go run ./cmd/notifications
```
