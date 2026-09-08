# KAMIS-BE-GO

Go rewrite of the KAMIS backend (migrated from `../KAMIS-BE`, Java/Spring Boot).
Single Go module monorepo: shared libraries in `pkg/`, one deployable per
directory under `services/`. Web framework: **Gin**. Auth: self-issued **RS256
JWT** via `golang-jwt/jwt/v5` — `profile` signs with the private key, every
other service verifies locally against the public one. The keypair is this
repo's own: tokens are not interchangeable with Java-issued ones, and the two
stacks are not meant to run side by side.

**Porting a service?** Read [MIGRATION.md](MIGRATION.md) first — it records the
locked decisions, migration order, and the behavioral contract quirks (role
casings, token shape, response envelope) that must be preserved.

## Layout

```
pkg/
  apierr/     the Invalid (400) / NotFound (404) error types handlers branch on
  auth/       JWT Verifier (all services) + Issuer (profile only) + Gin middleware
  blob/       image storage: GCS, or a local directory for development
  config/     env/.env loading
  database/   GORM Postgres connection + error translation
  httpx/      response envelope, CORS, logging, HTTP server, service-to-service client
  jsontime/   Jackson-compatible date rendering
  page/       the Spring Data Page JSON shape the frontend consumes
  reporting/  chart ranges, period labels and summary windows
services/
  profile/    auth, accounts, clients, suppliers  (port 8080)
  asset/      vehicles, photos, maintenance, reservations (port 8081)
  finance/    the ledger and the dashboards       (port 8082)
  project/    sales and distributions             (port 8083)
  purchase/   procurement, staged assets, reporting (port 8084)
  resource/   inventory catalogue                 (port 8085)
  template/   copyable skeleton service (sample "Resource" domain)
    cmd/                       entrypoint (main.go)
    internal/
      migrations/              schema history, embedded .sql files
      model/                   GORM entities      (≈ JPA @Entity / model)
      repository/              data access         (≈ Spring Data repository)
      service/                 business logic      (≈ @Service / restservice)
      handler/                 HTTP handlers       (≈ @RestController)
      router/                  routes + auth rules (≈ WebSecurityConfig)
    .env.example
    Dockerfile
```

The layered structure under `internal/` mirrors the Java package layout
(`restcontroller` / `restservice` / `repository` / `model`) so ports are
near-mechanical.

## Checks

```bash
make verify   # fmt-check + tidy-check + vet + lint + test — what CI runs
make fmt      # rewrite formatting in place
```

## Run a service locally

```bash
make tidy                              # download deps, generate go.sum (run once)
cp services/resource/.env.example services/resource/.env    # then fill JWT_PUBLIC_KEY
make run SERVICE=resource              # default is SERVICE=template
curl localhost:8085/health
```

## Add a new service

1. `cp -r services/template services/<name>` and rename the package paths.
2. Replace the sample `Resource` domain with the real entities, and replace
   `internal/migrations/00001_baseline.sql` with your own baseline.
3. Update routes + role guards in `internal/router/router.go` to match the
   service's `WebSecurityConfig` rules.
4. Only `profile` additionally constructs `auth.NewIssuer(...)` and owns the
   login, refresh and logout routes — every other service is verify-only.

## Notes

- `DATABASE_URL` is a Go/pgx DSN (`postgres://...`), **not** a JDBC URL.
- `JWT_PUBLIC_KEY` holds the one key a service verifies against. To rotate a
  keypair, set `JWT_PUBLIC_KEYS` instead — a comma-separated list, current key
  first — so the old and new keys need not be swapped in the same instant
  across every service. Each `.env.example` spells out the three steps.
- Schema is owned by **goose**: plain SQL under each service's
  `internal/migrations`, embedded in the binary and applied at start by
  `database.Migrate`. Add the next numbered file to change a schema; never edit
  one that has shipped.
- Build images from the repo root: `docker build -f services/<name>/Dockerfile
  --build-arg SERVICE=<name> .`
- Services serve through `httpx.Serve`, which applies HTTP timeouts and drains
  in-flight requests on SIGTERM. Logs are structured JSON (`log/slog`); set
  `LOG_LEVEL=debug` to widen.
