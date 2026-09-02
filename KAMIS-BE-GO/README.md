# KAMIS-BE-GO

Go rewrite of the KAMIS backend (migrated from `../KAMIS-BE`, Java/Spring Boot).
Single Go module monorepo: shared libraries in `pkg/`, one deployable per
directory under `services/`. Web framework: **Gin**. Auth: self-issued **RS256
JWT** via `golang-jwt/jwt/v5`, token-compatible with the legacy Java services so
both stacks can run side by side during the migration.

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
services/
  profile/    auth, accounts, clients, suppliers  (port 8080)
  asset/      vehicles, photos, maintenance, reservations (port 8081)
  resource/   inventory catalogue                 (port 8085)
  template/   copyable skeleton service (sample "Resource" domain)
    cmd/                       entrypoint (main.go)
    internal/
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
make verify   # fmt-check + vet + lint + test — what CI runs
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
2. Replace the sample `Resource` domain with the real entities.
3. Update routes + role guards in `internal/router/router.go` to match the
   service's `WebSecurityConfig` rules.
4. Only `profile` additionally constructs `auth.NewIssuer(...)` and exposes a
   login route — every other service is verify-only.

## Notes

- `DATABASE_URL` is a Go/pgx DSN (`postgres://...`), **not** a JDBC URL.
- `AutoMigrate` stands in for Hibernate `ddl-auto: update` during the migration;
  move to `goose`/`golang-migrate` for production-grade schema control.
- Build images from the repo root: `docker build -f services/<name>/Dockerfile
  --build-arg SERVICE=<name> .`
- Services serve through `httpx.Serve`, which applies HTTP timeouts and drains
  in-flight requests on SIGTERM. Logs are structured JSON (`log/slog`); set
  `LOG_LEVEL=debug` to widen.
