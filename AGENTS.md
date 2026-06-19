# Repository Guidelines

## Project Structure & Module Organization
- `cmd/server`: Go entrypoint that loads config, opens the DB, starts background scanners, and serves the Gin API plus optional static frontend.
- `cmd/javprovider`: small Go utility for exercising JAV provider lookups outside the main server.
- `internal/`: application packages; key folders include `common` (shared config/globals/logging), `db` (GORM setup and stores), `models` (domain structs), `server` (Gin router and handlers), `service` (directory/video/JAV scanners), `jav` (metadata providers), `manager` (cover and screenshot helpers), `mpv` (player integration), and `util` (filesystem, locale, proxy, playback, and platform helpers).
- `web/`: React + Tailwind frontend (Vite). Components live in `web/src/components`, shared state in `web/src/store.js`, API helpers in `web/src/api.js`, constants in `web/src/constants`, utilities in `web/src/utils`, and static icons in `web/public/ico`.
- `scripts/`: development/release helpers. `scripts/cli.sh` wraps common workflows, while `scripts/cli/` contains the Node CLI source and generated `scripts/cli/build/` output.
- `bin/`, `internal/bin/`, and `modernz/`: bundled runtime player/tool assets used by releases and mpv integration.
- `data/`, `web/dist/`, `release/`, `screenshot/`, `temp/`, `.gocache/`, and other scratch/build output directories are generated or runtime artifacts; keep them out of commits unless a release workflow explicitly requires them.

## Build, Test, and Development Commands
- Backend: `go run ./cmd/server -addr :17654 -static web/dist` to serve API (and built frontend when desired).
- Dev helper: `scripts/cli.sh dev backend|frontend` (flags: `WITH_STATIC=1`, `SKIP_NPM_INSTALL=1`, etc.).
- Tests: `GOCACHE=$(pwd)/.gocache go test ./...` (no Go tests yet—keep it green).
- Frontend (in `web/`): `npm install`; `npm run dev` for Vite, `npm run lint`, `npm run build` for prod bundle.
- CLI build: `cd scripts/cli && npm install && npm run build` (outputs `scripts/cli/build/javboss-cli.cjs`).
- Release: `scripts/cli.sh release linux-x86_64 v0.1.0` builds backend + `web/dist` and archives to `release/`.

## Coding Style & Naming Conventions
- Go: run `gofmt -w` and keep imports/go mod tidy; use context as first arg, return wrapped errors with lower-case messages, and log via `internal/common/logging`. Keep package names lowercase and files scoped to their package.
- Frontend: functional React components in PascalCase (`VideoGrid.jsx`), hooks/helpers camelCase. Keep styles in Tailwind/`index.css`; prefer colocated component styles. Format with `npm run format` / `npm run format:check`; lint with `npm run lint`.
- Naming: API routes are RESTful (`/videos`, `/tags`, `/directories`); keep new endpoints consistent and document query params.

## Database Migration Guidance
- Do not modify existing DB migration files. Add a new migration for every schema or data migration change, and use that new migration to reconcile schema details such as column order when needed.
- After every DB migration, verify that the actual database schema exactly matches the structs and GORM tags in `internal/models`, including tables, columns, column types, nullability/defaults, indexes, unique constraints, foreign keys, and join tables.
- Treat any drift between migrated schema and model definitions as a migration bug. Update the migration or model definitions until they are fully aligned before considering the change complete.

## Testing Guidelines
- Go: add table-driven `_test.go` files near the code under test; prefer integration tests around `internal/db` and handler tests via `httptest`. Use the repo-local `GOCACHE` path.
- Frontend: no unit tests today; at minimum run `npm run lint` and `npm run build` before PRs. When adding tests, colocate under `web/src` using Jest/Vitest conventions and match component names.
