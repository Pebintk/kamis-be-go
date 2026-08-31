# KAMIS Java → Go migration guide

How `../KAMIS-BE` (6 Spring Boot 3.4 / Java 21 microservices) is being ported to
this repo. `README.md` covers the repo layout and the mechanical "add a new
service" steps; this file records the **decisions, migration strategy, and
behavioral contract quirks** that must be preserved — the things you can't
rediscover from this repo alone.

## Status

| Service | Java port | Status |
|---|---|---|
| profile | 8080 | **Ported.** Slice 1: auth + account core (login, JWT issuance, add/list/paginate/update accounts, admin seeding). Slice 2: Client CRUD. Slice 3: Supplier CRUD, including the cross-service calls to project/resource/asset/purchase. |
| resource | 8085 | Not started (`services/template`'s sample domain is modeled on it). |
| asset | 8081 | Not started. |
| finance.report | 8082 | Not started. |
| project | 8083 | Not started. |
| purchase | 8084 | Not started. |

## Locked decisions

Don't relitigate these — they were weighed deliberately:

- **Framework: Gin** (over Chi). Gin's `binding:"required"` + `ShouldBindJSON`
  maps closely to Spring's `@Valid @RequestBody`, minimizing porting friction
  for the DTO-heavy controllers.
- **Auth: self-issued JWT, NOT an external IdP.** KAMIS is the only consumer;
  Keycloak/Zitadel/Ory operational cost isn't justified. `golang-jwt/jwt/v5`
  replaces the hand-rolled per-service `JwtUtils`/`JwtTokenFilter`.
- **Token compatibility is the linchpin.** Keep RS256 + the *existing* RSA
  keypair + the exact claim shape (`sub` = username, single `role` claim,
  `iat`, `exp` — nothing else). This makes Java- and Go-issued tokens
  interchangeable, which is what lets the strangler migration work. Env
  formats are reused as-is: `JWT_PUBLIC_KEY` = base64 X509
  (`ParsePKIXPublicKey`), `JWT_SECRET_KEY` = base64 PKCS8
  (`ParsePKCS8PrivateKey`).
- **Passwords: reuse existing BCrypt hashes** via `golang.org/x/crypto/bcrypt`
  — zero password migration.
- **ORM: GORM** with `AutoMigrate` standing in for Hibernate
  `ddl-auto: update` during the migration. Move to `goose`/`golang-migrate`
  before this becomes the system of record for schema.
- **`DATABASE_URL` is a Go/pgx DSN** (`postgres://...`), not a JDBC URL.
- **Single-role claim.** The Java code only reads the FIRST authority into the
  `role` claim, so the port is faithfully single-role. Multi-role is a
  deferred enhancement (Phase 3).

## Migration strategy: strangler, service by service

Run both stacks side by side on the `kamis-network` Docker network behind a
reverse proxy (Traefik/Caddy/nginx). The FE already calls each service through
its own `VITE_API_*_URL`, so cutting over one service = repointing one route.
No big-bang switch.

Recommended order:

1. **Prove the pattern on a simple leaf *verifier* service** (resource or
   finance.report) — verify-only auth, no dependents.
2. Migrate outward: asset, purchase, project.
3. **Migrate profile (the issuer) LAST**, keeping the token contract
   byte-identical the whole time.

(In practice profile was ported *first* at the user's request to prove the
issuer works — but it should be *cut over* last, and its DB flatten (below)
means it can't share the legacy DB live.)

## Contract quirks — verified against the Java source + FE auth store

These are behaviors the FE and the other services depend on. Breaking any of
them breaks the strangler migration.

### Roles have THREE casings

| Context | Casing | Example |
|---|---|---|
| DB `user_type` column | UPPERCASE | `ADMIN` |
| JWT `role` claim + FE route guards | PascalCase (legacy `getClass().getSimpleName()`) | `Admin` |
| API request/response bodies | lowercase | `admin` |

The canonical mapping lives in
`services/profile/internal/model/user.go` (`ROLES` table). Roles: Admin,
Operasional, Finance, Direksi (+ the EndUser base concept).

### Auth & token shape

- **Login is BY EMAIL**; the token `sub` is the **username**.
- Token carries only `sub` + `role` + `iat` + `exp`. The FE's
  `decoded.id`/`decoded.email` are *already undefined today* — do NOT "fix"
  this by adding claims.

### Response envelope

Every response is wrapped in the Java `BaseResponseDTO` shape
(`status`/`message`/`timestamp`/`data`) — in Go, `pkg/httpx`'s
`Envelope`/`Respond`. The FE reads `response.data.data` (e.g. login token at
`response.data.data.token`).

### Profile route rules (from the Java `WebSecurityConfig`)

- `/api/auth/login` and `/api/profile/add` are **public** (yes, add is public).
- `/api/profile/all` is **Admin-only**.
- Other `/api/profile/**` routes: all four roles.
- `PUT /api/profile/{id}` updates **by EMAIL** — the path value is an email
  address, not a UUID.

### Schema flatten (profile only)

Java modeled roles with JPA JOINED inheritance: an `end_user` table plus
*empty* `Admin`/`Operasional`/`Finance`/`Direksi` child tables keyed by a
discriminator. The Go port collapses this to a **single `end_user` table with a
`user_type` column**. Consequence: profile needs a **one-time data-flatten
migration** from the legacy DB rather than live DB-sharing — the one place the
issuer-first port bites. `user_type` keeps the legacy UPPERCASE values so the
flatten is trivial.

### Three /api/client routes are unauthenticated — deliberately

`WebSecurityConfig` lists rules for `/api/client/all` and `/api/client/add` but
has **no `/api/client/**` catch-all**, so everything else under `/api/client`
falls through to `.anyRequest().permitAll()`:

- `GET /api/client/all/paginated`
- `GET /api/client/{id}`
- `PUT /api/client/update/{id}`

This is load-bearing, not a porting oversight: the frontend's `getClientDetail`
and `updateClient` (`src/stores/client.ts`) send **no Authorization header at
all**, so requiring a token would break the client detail and edit pages. The Go
router reproduces it and says so in a comment. `PUT /api/client/update/{id}`
being world-writable is worth fixing — but fix it in the frontend and backend
together, as one change, not during the cutover.

(Related oddity: `GET /api/client/all` *is* role-guarded, yet the frontend's
`viewAllClient` sends no token either — so that call already 403s today. The Go
port keeps the guard.)

### Column names are quoted identifiers with spaces

The `Client` and `Supplier` JPA entities declare columns like
`@Column(name = "Nomor Telepon")` and `@Column(name = "Created Date")`. Those
are not valid unquoted SQL identifiers, so Hibernate emits them quoted, and they
exist in the live database exactly as spelled — spaces, capitals and all. The
GORM tags mirror them, and GORM quotes identifiers the same way. **Verify with
`\d "Client"` and `\d "Supplier"` in psql before cutting over**; this is the
one detail that would silently read the wrong (or no) column.

### Intentional divergences from Java

- `/api/profile/add` returns the created user DTO (no password) instead of
  echoing the request back **with the BCrypt hash in it**, as Java did.
- `filterClientsPaginated` in Java mapped clients outside the `minProfit` /
  `maxProfit` range to **null** instead of dropping them, so the page's
  `content` array came back padded with nulls. The Go version drops them. Page
  metadata still describes the unfiltered query, as in Java, because the profit
  is computed from the project service and cannot be expressed in the SQL count.
- Client list endpoints fetch each client's projects **concurrently** (bounded
  at 8) rather than one at a time. Same JSON, far fewer sequential round trips.
- `activityName` tolerates a null `purchasePrice` (renders `Rp0`) where the Java
  code would have thrown a NullPointerException.

## Porting recipe (per service)

1. `cp -r services/template services/<name>`, rename package paths (see
   README "Add a new service").
2. Port entities from `model/` (JPA) → `internal/model/` (GORM), preserving
   table/column names so the Go service can point at the existing database.
3. Port `restservice/` → `internal/service/`, `restcontroller/` →
   `internal/handler/`, DTOs from `restdto/` → `internal/dto/`.
4. Translate the service's `WebSecurityConfig` path rules into
   `internal/router/router.go` (`GinAuth()` + `GinRequireRole(...)` replace
   `hasAnyAuthority(...)`). Keep the PascalCase role names.
5. Wrap all responses with `httpx.Respond` and compare JSON output
   field-by-field against the running Java service — the FE parses these
   shapes directly.
6. Inter-service calls forward the caller's JWT (the Java services use
   WebClient this way; keep the same behavior).
7. `go build ./...`, `go vet ./...`, `go test ./...` before calling a slice
   done.

## Notes for the next port

`pkg/httpx.Client` is the Go stand-in for Spring's `WebClient`: it forwards the
caller's bearer token, unwraps the `BaseResponseDTO` envelope via
`httpx.GetData[T]`, and reports a downstream 404 as `httpx.ErrNotFound` so
callers can treat it as "no data" the way the Java `onStatus(...)` handlers did.
The token reaches it from `auth.ForwardToken()`, which every service should
install globally — including on public routes, since the legacy services forward
whatever token they were handed regardless of whether the route required one.

## Deferred (Phase 3 — after profile is fully ported)

These all change the token contract, so they wait until nothing depends on the
legacy Java issuer:

- Refresh tokens.
- `jti` + revocation list (real logout).
- Multi-role support.
- JWKS endpoint on profile for key rotation.
- Decide schema ownership: replace `AutoMigrate` with goose/golang-migrate, or
  freeze the schema.
