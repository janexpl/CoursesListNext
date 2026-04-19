# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Full-stack training management system (courses, students, companies, certificates, training journals) with a Go REST API backend and Nuxt 4 frontend, backed by PostgreSQL.

## Architecture

- **`api/`** — Go REST API using chi router, sqlc for type-safe DB queries, session-based auth with cookie tokens
- **`web/`** — Nuxt 4 (Vue 3) frontend with Tailwind CSS 4, @nuxt/ui components, TypeScript
- **`docker-compose.yml`** (parent dir) — orchestrates PostgreSQL 16, API (port 8081), and Web (port 3000)

### Data flow

Frontend (`useApi()` composable) → Nuxt server proxy (`server/api/[...path].ts`) → Go API at `http://127.0.0.1:8081/api/v1/*`. Cookies are forwarded for session auth.

### Backend patterns (`api/internal/`)

Each domain (auth, users, students, companies, courses, certificates, journals, registries, dashboard) follows a **handler → service → sqlc** layered pattern:

- **Handler** (`handler.go`): Thin HTTP layer — parses request, calls service/queries, writes response. Accepts interfaces (not concrete types) for testability.
- **Service** (`service.go`): Business logic and transactions. Uses a `txScope` struct wrapping `*sqlc.Queries`, `commit`, and `rollback` functions.
- **DTOs** (`dto.go`, `create_dto.go`): Separate structs for list vs detail responses. JSON tags use camelCase. Request payloads use embedded base structs.
- **Database**: Queries in `db/queries/*.sql`, generated code in `db/sqlc/`, schema in `db/schema.sql`, migrations in `api/migrations/` (forward-only, numbered).

### Transaction pattern

Services use a committed flag with deferred rollback:
```go
defer func() {
    if !committed {
        if err := tx.rollback(ctx); err != nil {
            log.Printf("unable to rollback changes: %v", err)
        }
    }
}()
```

### Error handling

- Sentinel errors: `ErrInvalidInput`, `ErrNotFound` defined at package level
- Response codes: `bad_request`, `unauthorized`, `not_found`, `internal_error`, `conflict`, `forbidden`
- `response.WriteJSON(w, statusCode, data)` for success, `response.WriteError(w, statusCode, code, message)` for errors
- `response.WriteNoContent(w)` for 204 responses
- `response.ParsePositiveInt64PathValue(r, "id")` for route param extraction

### Audit logging

Inside transactions, call `s.recorder.Record(ctx, tx.queries, auditlog.Entry{...})` with `EntityType`, `EntityID`, `Action` ("create"/"update"/"delete"), `Before`/`After` snapshots.

### Auth

Session-based with HTTP-only cookies. Roles: regular user and admin (role=1). Frontend middleware: `auth`, `guest`, `admin`. Use `auth.UserFromContext(ctx)` to get the authenticated user in handlers.

### Shared helpers

- `response.HandleDBError(w, err, entityName)` — handles `pgx.ErrNoRows` (404) vs internal error (500)
- `response.ParseListParams(r)` — parses `search` and `limit` query params, returns `(pgtype.Text, int32, error)`. Limit range: 1–100, default 50.
- `response.DateFormat` / `response.TimestampzFormat` — date format constants
- `validation.CheckEmail(email)` — email format validation using `net/mail`
- `pgutil` — nullable type conversions: `NullableDate()`, `NullableTimestampz()`, `NullableString()`, `OptionalText()`, `OptionalInt8()`

### Frontend patterns

- **`useApi()`**: All API calls go through this composable. Handles cookie forwarding on SSR, 401 redirect to `/login`, typed methods per endpoint.
- **`useAuth()`**: Global auth state via `useState()`. Methods: `fetchMe()`, `login()`, `logout()`. Computed: `isAuthenticated`.
- **`getApiErrorMessage(error, fallback)`**: Extracts `error.data.error.message` from API error responses.
- **Pages**: Reactive forms with `reactive()`, `submitPending` ref for loading state, tab-based section navigation.

## Development Commands

### Frontend (`web/`)

```bash
pnpm install          # Install dependencies (uses pnpm)
pnpm dev              # Dev server at http://localhost:3000
pnpm build            # Production build
pnpm lint             # ESLint
pnpm typecheck        # TypeScript type checking
```

### Backend (`api/`)

```bash
go run ./cmd/api                              # Run API server
go build -o api ./cmd/api                     # Build binary
go test ./...                                 # Run all tests
go test ./internal/courses -run TestListCourses -v  # Run a single test
go test ./internal/courses -v                 # Run all tests in a package
```

### SQLC workflow

```bash
cd api && sqlc generate    # Regenerate after editing db/queries/*.sql or db/schema.sql
```

### Full stack (from parent dir)

```bash
docker-compose up     # Start db + api + web
```

## Code Style

### Frontend
- No semicolons, single quotes, no trailing commas, 100 char print width (`.prettierrc.json`)
- ESLint config in `eslint.config.mjs`
- UI colors: primary=sky, neutral=slate

### Backend
- Standard Go conventions
- sqlc for all database queries — edit `.sql` files then regenerate, don't modify `db/sqlc/*.go` directly
- Always `return` after `response.WriteError()` — forgetting this causes double-write bugs
- Log errors from `w.Write()` and `tx.Rollback()` instead of ignoring with `_ =`
- Request body decoding: always call `decoder.DisallowUnknownFields()` before decoding
- Tests use fake implementations of handler interfaces (not mocks), with function fields for per-test behavior

## Environment Variables

Backend (`api/.env`): `PORT`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `DB_SSLMODE`, `SESSION_COOKIE_NAME`, `SESSION_TTL`, `SESSION_COOKIE_SECURE`, `CORS_ALLOWED_ORIGINS`, `LOGIN_RATE_LIMIT`

Frontend: `NUXT_API_TARGET` (default `http://127.0.0.1:8081`)

## Key Notes

- UI is in Polish language
- PDF generation uses wkhtmltopdf/Chromium (Docker environment)
- Certificate registry numbers are auto-incremented per course per year
- App uses search with limits instead of pagination (no offset needed)
- Healthcheck endpoint: `GET /api/v1/healthz`
