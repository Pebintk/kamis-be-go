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
  auth/       JWT Verifier (all services) + Issuer (profile only) + Gin middleware
  config/     env/.env loading
  database/   GORM Postgres connection
  httpx/      shared JSON response helpers
services/
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

## Run a service locally

```bash
make tidy                              # download deps, generate go.sum (run once)
cp services/template/.env.example services/template/.env   # then fill JWT_PUBLIC_KEY
make run                               # runs the template service (SERVICE=template)
make run SERVICE=resource              # once other services exist
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
