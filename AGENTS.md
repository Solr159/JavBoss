# Repository Guidelines

## Project Structure

### Backend (Go)

- `cmd/server`: Go entrypoint that loads config, opens the DB, starts background scanners, and serves the Gin API plus optional static frontend.
- `cmd/javprovider`: small Go utility for exercising JAV provider lookups outside the main server.
- `internal/`: application packages; key folders include `common` (shared config/globals/logging), `db` (GORM setup and stores), `models` (domain structs), `server` (Gin router and handlers), `service` (directory/video/JAV scanners), `jav` (metadata providers), `manager` (cover and screenshot helpers), `mpv` (player integration), and `util` (filesystem, locale, proxy, playback, and platform helpers).

### Frontend (React + Tailwind, Vite)

The frontend lives in `web/`; paths below are relative to that directory.

- `src/features/<domain>/`: business components in `components/`, business hooks in `hooks/`, and API helpers in `api.js`. Place code by business ownership, including components reused across domains.
- `src/shared/ui` and `src/shared/hooks`: domain-independent UI and hooks only. They must not import feature modules, application state, or API clients; pass business data and callbacks through props or hook arguments.
- `src/app/layout`: application layout. `src/routes`: route composition.
- `src/navigation`: navigation, history, scroll restoration, and page navigation shortcuts. Keep navigation contexts beside their navigation hooks. `src/query`: data loading coordination.
- `src/store.js` and `src/state`: shared state composition and modules. `src/api.js` remains a compatibility barrel for feature API helpers.
- `src/constants` and `src/utils`: constants and utilities. `public/ico`: static icons.
- `tests/`: frontend tests, mirroring source organization where practical.
- Do not recreate top-level `src/components` or `src/hooks` directories; use the ownership boundaries above.

### Tooling, Assets, and Generated Files

- `scripts/`: development/release helpers. `scripts/cli.sh` wraps common workflows, while `scripts/cli/` contains the Node CLI source and generated `scripts/cli/build/` output.
- `bin/`, `internal/bin/`, and `modernz/`: bundled runtime player/tool assets used by releases and mpv integration.
- `data/`, `web/dist/`, `release/`, `screenshot/`, `temp/`, `.gocache/`, and other scratch/build output directories are generated or runtime artifacts; keep them out of commits unless a release workflow explicitly requires them.

## Build, Test, and Development Commands
- Backend: `go run ./cmd/server` to serve API on port 17654 (and `web/dist` when present).
- Dev helper: `scripts/cli.sh dev backend|frontend` (flags: `SKIP_NPM_INSTALL=1`, etc.).
- Tests: `GOCACHE=$(pwd)/.gocache go test ./...` (no Go tests yet—keep it green).
- Frontend (in `web/`): `npm install`; `npm run dev` for Vite, `npm run lint`, `npm run build` for prod bundle.
- CLI build: `cd scripts/cli && npm install && npm run build` (outputs `scripts/cli/build/javboss-cli.cjs`).
- Release: `scripts/cli.sh release linux-x86_64 v0.1.0` builds backend + `web/dist` and archives to `release/`.

## Coding Style & Naming Conventions
- Go: run `gofmt -w` and keep imports/go mod tidy; use context as first arg, return wrapped errors with lower-case messages, and log via `internal/common/logging`. Keep package names lowercase and files scoped to their package.
- Frontend: functional React components in PascalCase (`VideoGrid.jsx`), hooks/helpers camelCase. Keep styles in Tailwind/`index.css`; prefer colocated component styles. Format with `npm run format` / `npm run format:check`; lint with `npm run lint`.
- Naming: API routes are RESTful (`/videos`, `/tags`, `/directories`); keep new endpoints consistent and document query params.

## Frontend Implementation Guidelines
- Prefer built-in capabilities of React, the existing component libraries (such as MUI), and the browser/platform. Check existing props, hooks, native HTML behavior, and CSS before implementing custom behavior.
- Prefer configuration and composition over reimplementing framework or platform behavior. Avoid custom event tracking, focus management, timers, or state machines when existing capabilities meet the requirement.
- When custom logic is necessary, keep it minimal and local, explain the specific gap it addresses, and preserve built-in accessibility and interaction behavior. Do not turn a local UI fix into a general-purpose mechanism without a concrete need.

## README Guidelines
- `README.md` is for end users. Keep edits concise and restrained, focusing on installation, configuration, usage, and limitations that affect user decisions.
- Update it only when a change materially affects users; routine implementation changes do not need a README entry. Prefer small edits to existing sections and preserve the document's language and style.
- Avoid implementation details, API field inventories, debugging notes, and exhaustive technical explanations. Put necessary developer documentation elsewhere and link to it only when useful to users.

## Database Migration Guidance
- Do not modify existing DB migration files. Add a new migration for every schema or data migration change, and use that new migration to reconcile schema details such as column order when needed.
- After every DB migration, verify that the actual database schema exactly matches the structs and GORM tags in `internal/models`, including tables, columns, column types, nullability/defaults, indexes, unique constraints, foreign keys, and join tables.
- Treat any drift between migrated schema and model definitions as a migration bug. Update the migration or model definitions until they are fully aligned before considering the change complete.

## Testing Guidelines
- Go: add table-driven `_test.go` files near the code under test; prefer integration tests around `internal/db` and handler tests via `httptest`. Use the repo-local `GOCACHE` path.
- Frontend: keep all frontend tests under `web/tests/`; Mirror the source organization and names under `web/tests/` when practical (for example, `web/src/utils/javEdit.js` should be tested by `web/tests/utils/javEdit.test.js`). Run `npm test`, `npm run lint`, `npm run format:check`, and `npm run build` before PRs.
