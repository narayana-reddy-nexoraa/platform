# Repository Guidelines

## Project Structure & Module Organization
`cmd/api`, `cmd/worker`, and `cmd/temporal-worker` are the three Go entrypoints. Core backend code lives in `internal/` by layer: `handler/`, `service/`, `repository/`, `broker/`, `temporal/`, `worker/`, and `sop/`. Database migrations and SQL sources live in `db/migrations/` and `db/queries/`; sqlc generates code into `internal/repository/db/`. Frontend code is in `ui/src`. Cross-cutting infrastructure lives under `deploy/`, `docker-compose.yml`, `terraform/`, `k8s/`, `argocd/`, `crossplane/`, and `backstage/`. Tests are split into `tests/integration/`, `tests/e2e/`, `tests/k6/`, and `tests/load/`.

## Build, Test, and Development Commands
- `make docker-up`: start the full local Docker stack.
- `make migrate-up`: apply DB migrations against local Postgres.
- `make run-api` / `make run-worker`: run Go services locally against supporting dependencies.
- `make build-api && make build-worker && make build-temporal-worker`: compile all binaries into `bin/`.
- `make test-unit`, `make test-integration`, `make test-e2e`, `make test-compliance`, `make test-load-sop`: run focused backend checks.
- `cd ui && npm install && npm run dev`: start the Vite UI on `localhost:3000`.
- `cd ui && npm run build`: type-check and build the frontend bundle.

## Coding Style & Naming Conventions
Use standard Go formatting with `goimports`; package names stay lowercase, exported identifiers use `CamelCase`, and tests use `*_test.go`. Keep repository-layer mapping logic in `internal/repository/*`, not in handlers. For React/TypeScript, keep components in `PascalCase` and follow the existing Vite/React 19 structure in `ui/src`. Never edit `internal/repository/db/` directly; change `db/queries/*.sql` and run `make sqlc-generate`.

## Testing Guidelines
Prefer table-driven Go tests for services and repositories. Run the narrowest relevant target first, then broader suites if the change crosses layers. Integration and E2E tests require Docker-backed dependencies. If you touch SQL, migrations, or repository code, re-run `make sqlc-generate` and at least `make test-unit`.

## Commit & Pull Request Guidelines
Recent history follows conventional commits such as `feat:` and `fix:`. Keep messages imperative and concise, for example `fix: correct docker UI port in README`. PRs should include a short summary, linked issue or sprint item, validation commands run, and screenshots for UI changes. Call out schema, migration, or infrastructure impact explicitly.

## Security & Configuration Tips
Start from `.env.example`; do not commit real secrets. Avoid rewriting existing numbered migrations once shared; add a new migration instead. Use tenant-aware API requests in local testing by setting `X-Tenant-ID` to a valid UUID.
