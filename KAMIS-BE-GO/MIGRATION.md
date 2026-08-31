# KAMIS Java → Go migration guide

How `../KAMIS-BE` (6 Spring Boot 3.4 / Java 21 microservices) is being ported to
this repo. `README.md` covers the repo layout and the mechanical "add a new
service" steps; this file records the **decisions and the contract that has to
hold** — the things you can't rediscover from this repo alone.

## What this migration is (and isn't)

**The KAMIS app is not in use.** There are no live users, no production data
worth keeping, and no requirement to run the Java and Go stacks side by side.
This is a rewrite for its own sake, not a live cutover.

That single fact removes most of what would otherwise constrain the port, so
don't reintroduce those constraints:

| Still matters | No longer matters |
|---|---|
| The **JSON contracts** — the `BaseResponseDTO` envelope, field names, role casings, login-by-email. The unchanged Vue frontend is the real compatibility target. | Reading Hibernate's existing tables. Schema is ours; `AutoMigrate` builds it fresh. |
| The **route paths**, for the same reason. | Token interchangeability with Java-issued tokens. A fresh RSA keypair is fine. |
| | Reverse-proxy strangler machinery. To adopt a Go service, repoint one `VITE_API_*_URL`. |
| | Porting order. `profile` needn't go last; port in whatever order is most useful. |
| | Reproducing Java's bugs. Fix them and note the fix. |

If the app ever does go live on the Java stack first, revisit this section
before continuing — most of the guidance below would change.

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
- **Token claim shape follows the frontend, not the Java service.** RS256 with
  `sub` = username and a single `role` claim, because that is what the Vue auth
  store decodes. Interchangeability with Java-issued tokens is no longer a
  requirement, so a freshly generated RSA keypair is fine. Env formats are kept
  as-is out of convenience: `JWT_PUBLIC_KEY` = base64 X509
  (`ParsePKIXPublicKey`), `JWT_SECRET_KEY` = base64 PKCS8
  (`ParsePKCS8PrivateKey`).
- **Passwords: BCrypt** via `golang.org/x/crypto/bcrypt`, the same algorithm
  Java used, so any old hash would still verify if data ever were imported.
- **ORM: GORM**, with `AutoMigrate` building the schema. Move to
  `goose`/`golang-migrate` before this ever holds real data.
- **`DATABASE_URL` is a Go/pgx DSN** (`postgres://...`), not a JDBC URL.
- **Single-role claim.** Java only read the first authority into the `role`
  claim and the frontend's route guards assume one role, so the port is
  single-role. Multi-role is a deferred enhancement.

## Migration approach

Port one service at a time, copying `services/template`. When a service is
ready, point the frontend's `VITE_API_*_URL` for that domain at it — there is no
proxy layer and no coordination window, because nothing is serving traffic.

Suggested order, easiest first: `resource` or `finance.report` (leaf,
verify-only), then `asset`, `purchase`, `project`. `profile` is already done.

## Contract quirks — verified against the Java source + FE stores

These are behaviours the **frontend** depends on. Breaking them means changing
the Vue app too.

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

### Route rules

Only two routes are public: `POST /api/auth/login` and `POST /api/profile/add`
(user registration was public in Java and stays that way — worth revisiting if
this app is ever exposed). Everything else needs a valid token, plus a role:

| Route | Roles |
|---|---|
| `GET /api/profile/all` | Admin |
| other `/api/profile/**` | all four |
| `GET /api/client/**` | all four |
| `POST /api/client/add` | Operasional |
| `PUT /api/client/update/{id}` | Operasional, Admin |
| `POST /api/supplier/add` | Operasional |
| `PUT /api/supplier/update`, `/add-purchase` | Operasional, Admin |
| other `/api/supplier/**` | all four |

`PUT /api/profile/{id}` updates **by EMAIL** — the path value is an email
address, not a UUID. That one is inherited from Java and the frontend relies on
it.

### Role storage is flattened

Java modelled roles with JPA JOINED inheritance: an `end_user` table plus
*empty* `Admin`/`Operasional`/`Finance`/`Direksi` child tables keyed by a
discriminator. The Go port collapses that to a **single `end_users` table with a
`user_type` column**. `user_type` keeps the UPPERCASE values because the role
casing map (above) is built around them.

### Fixed: three /api/client routes had no auth

`WebSecurityConfig` had rules for `/api/client/all` and `/api/client/add` but no
`/api/client/**` catch-all, so `GET /client/all/paginated`, `GET /client/{id}`
and `PUT /client/update/{id}` fell through to `.anyRequest().permitAll()` —
world-readable, and world-*writable* for the update. The Vue client store
matched, sending no `Authorization` header on those calls.

Both sides are fixed. The Go router guards every `/api/client` route (reads =
all four roles, `add` = Operasional, `update` = Operasional/Admin, mirroring the
supplier rules), and the frontend now attaches the token through a single axios
interceptor in `src/config/http.ts`, installed from `main.ts`. That replaces
per-call headers that individual stores could forget — which is how the gap
arose. `router_test.go` asserts the routes stay guarded.

### Naming: schema is idiomatic, not inherited

The Java `Client` and `Supplier` entities declared columns as quoted identifiers
with spaces and capitals — `@Column(name = "Nomor Telepon")`, `"Created Date"`,
tables `"Client"` / `"Supplier"`. The Go models originally mirrored that, purely
to stay readable against Hibernate's tables. Nothing needs that any more, so the
models carry **no explicit column or table names at all** and take GORM's
defaults: tables `clients`, `suppliers`, `end_users`, columns `name_client`,
`no_telp_client`, `created_at`, and so on. Any raw SQL fragment in a repository
uses those names.

The Go struct fields still read `NameClient`, `NoTelpClient` etc. so they line
up one-to-one with the DTO fields the frontend expects.

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
2. Port entities from `model/` (JPA) → `internal/model/` (GORM). Use GORM's
   default naming; do not carry over Hibernate's quoted column names.
3. Port `restservice/` → `internal/service/`, `restcontroller/` →
   `internal/handler/`, DTOs from `restdto/` → `internal/dto/`.
4. Translate the service's `WebSecurityConfig` path rules into
   `internal/router/router.go` (`GinAuth()` + `GinRequireRole(...)` replace
   `hasAnyAuthority(...)`). Keep the PascalCase role names.
5. Wrap all responses with `httpx.Respond` and check the JSON field-by-field
   against the Java DTOs (and the matching Vue store/interface) — the frontend
   parses these shapes directly.
6. Inter-service calls forward the caller's JWT (the Java services use
   WebClient this way; keep the same behavior).
7. `go build ./...`, `go vet ./...`, `go test ./...` before calling a slice
   done.

## Notes for the next port

Four shared pieces exist now; use them rather than reinventing per service.

**`httpx.Serve(engine, port)`** replaces gin's `Engine.Run`. It sets read/write/
idle timeouts (a bare `http.Server` has none, so one slow client can hold a
connection open forever) and drains in-flight requests on SIGTERM, which is what
lets `docker stop` roll a service without dropping requests.

**`httpx.SetupLogging(name)` + `httpx.RequestLogger()`** give structured JSON
logs via `log/slog`, one line per request, with `LOG_LEVEL` to widen. Call
SetupLogging first thing in `main`, and use RequestLogger in place of
`gin.Logger()` so request logs match the rest of the process's output.

**`database.Translate(err)`** maps `gorm.ErrRecordNotFound` and the Postgres
`23505` unique violation onto `database.ErrNotFound` / `database.ErrDuplicate`.
Every repository return goes through it, which keeps gorm out of the service
layer and detects duplicates via the typed `*pgconn.PgError` rather than
matching on message text. Services branch with `errors.Is`.

**`pkg/httpx.Client`** is the Go stand-in for Spring's `WebClient`: it forwards the
caller's bearer token, unwraps the `BaseResponseDTO` envelope via
`httpx.GetData[T]`, and reports a downstream 404 as `httpx.ErrNotFound` so
callers can treat it as "no data" the way the Java `onStatus(...)` handlers did.
The token reaches it from `auth.ForwardToken()`, which every service should
install globally — including on public routes, since the legacy services forward
whatever token they were handed regardless of whether the route required one.

## Tooling

`make verify` is what CI runs and what to run before pushing: `fmt-check`,
`vet`, `lint`, `test`. Lint is golangci-lint, pinned by version in the Makefile
and installed on demand into `bin/tools`. `.golangci.yml` disables exactly one
check, ST1005 (lowercase, unpunctuated error strings), because the supplier
flows return Indonesian sentences that the frontend renders verbatim — they are
API contract, not internal diagnostics.

**Gin is held at v1.10.x on purpose.** From v1.11 gin imports
`quic-go/http3`, which links an HTTP/3 stack none of these services use and adds
~5MB to every binary plus a large network-facing dependency to triage advisories
for. v1.10.1 is the current patch of that line and `govulncheck` reports no
vulnerabilities we call. Revisit if the 1.10 line stops getting fixes — a plain
`go get -u ./...` will jump to 1.12 and silently reintroduce it.

## Deferred

Nothing here is blocked any more — the Java stack is not a constraint. These are
just not done yet, roughly in order of how much they'd matter if the app were
ever used for real:

- Replace `AutoMigrate` with goose/golang-migrate before the schema holds data
  worth keeping.
- Refresh tokens, and `jti` + a revocation list so logout is real.
- Reconsider whether `POST /api/profile/add` should stay public.
- Multi-role support (the frontend assumes one role today).
- JWKS endpoint on profile for key rotation.
