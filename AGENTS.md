# Repository Guidelines

## Project Structure & Module Organization
This repository has two applications. `api/` is a Go service with the entry point at `api/cmd/api/main.go`. Keep feature code under `api/internal/<domain>/` (for example `courses`, `journals`, `users`), shared helpers under `api/internal/*util` or `api/internal/response`, SQL under `api/internal/db/queries/`, generated `sqlc` code under `api/internal/db/sqlc/`, and schema changes in `api/migrations/`.

`web/` is a Nuxt 4 frontend. Pages live in `web/app/pages/`, shared UI in `web/app/components/`, reusable client logic in `web/app/composables/`, route guards in `web/app/middleware/`, and server proxy code in `web/server/api/[...path].ts`. Static assets belong in `web/public/` or `web/app/assets/`.

## Build, Test, and Development Commands
- `cd web && pnpm install`: install frontend dependencies.
- `cd web && pnpm dev`: start the Nuxt app on `http://localhost:3000`.
- `cd web && pnpm build`: create a production frontend build.
- `cd web && pnpm lint`: run ESLint for Vue and TypeScript files.
- `cd web && pnpm typecheck`: run Nuxt type checking.
- `cd api && go run ./cmd/api`: start the API locally; it reads `.env` values for DB and session config.
- `cd api && go test ./...`: run backend unit tests.
- `docker compose -f docker-compose.prod.yml up -d`: start the production-like stack locally if needed.

## Coding Style & Naming Conventions
Frontend formatting is defined in `web/.prettierrc.json`: 2-space indentation, no semicolons, single quotes, no trailing commas, and `printWidth` 100. Use PascalCase for Vue components such as `AppHeader.vue`, `useX` names for composables, and Nuxt file-based route names under `web/app/pages/`.

Backend code should follow standard Go formatting with `gofmt`. Keep package names lowercase, keep handlers and DTOs inside their feature package, and avoid editing generated files in `api/internal/db/sqlc/` by hand.

## Testing Guidelines
Backend tests use Go's `testing` package and live beside the code as `*_test.go`. Add or update tests when changing handlers, services, or validation rules, especially for error responses and input parsing. There is no dedicated frontend test runner configured yet, so every web change should at least pass `pnpm lint` and `pnpm typecheck`.

## Commit & Pull Request Guidelines
Recent history uses short, direct subjects such as `healthcheck` and `build problem`. Follow that pattern: one-line, present-tense commit messages focused on a single change, with lowercase subjects preferred.

Pull requests should include a brief summary, note whether `api/`, `web/`, migrations, or environment variables changed, and include screenshots for UI updates. Call out any required `.env` or deployment changes explicitly.
