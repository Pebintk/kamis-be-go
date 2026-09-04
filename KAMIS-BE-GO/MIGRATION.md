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
| resource | 8085 | **Ported.** The full inventory catalogue: CRUD, the paginated/name-filtered list, the locked stock adjustments, the low-stock report, and the supplier-link endpoints profile calls. All 12 Java endpoints map 1:1. |
| asset | 8081 | **Ported.** Assets + photos (GCS via `pkg/blob`), maintenance scheduling with reservation-conflict checks, and project reservations. All 21 Java endpoints map 1:1. |
| finance.report | 8082 | **Ported** as `services/finance` (the dot does not belong in a Go import path). The ledger every other service pushes to, plus the dashboard charts and summaries. All 10 Java endpoints map 1:1. |
| project | 8083 | **Ported.** Sales and distributions, the status and payment workflow, and the activity charts. All 11 Java endpoints map 1:1. |
| purchase | 8084 | **Ported.** Purchase requests with resource line items or a staged asset, the status workflow with its role rules, and the chart/range/summary reporting. All 15 Java endpoints map 1:1. |

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
- **ORM: GORM**, with **goose** owning the schema. `AutoMigrate` built it from
  the structs at every start, which was right while the schema was still moving
  and nothing was stored, but it has no history, no down path, and no way to
  express anything the struct tags cannot say. Migrations are plain SQL under
  each service's `internal/migrations`, embedded in the binary, applied by
  `database.Migrate` at start.
- **`DATABASE_URL` is a Go/pgx DSN** (`postgres://...`), not a JDBC URL.
- **Single-role claim.** Java only read the first authority into the `role`
  claim and the frontend's route guards assume one role, so the port is
  single-role. Multi-role is a deferred enhancement.

## Migration approach

Port one service at a time, copying `services/template`. When a service is
ready, point the frontend's `VITE_API_*_URL` for that domain at it — there is no
proxy layer and no coordination window, because nothing is serving traffic.

**All six services are ported.** What remains is the deferred work below —
principally replacing `AutoMigrate` with real migrations before this holds data
worth keeping.

`finance.report` is a leaf but goes **last**: it only *reads*, and everything it
reads comes from `project` and `purchase`. Ported before them, its dashboard
endpoints have nothing to answer with. `asset` went earlier despite also having
an outbound call, because that call is a single fire-and-forget write to
finance's ledger that the legacy service already swallowed on failure — a
non-blocking dependency, where finance's are blocking.

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

The resource service has no public routes at all. Its rules:

| Route | Roles |
|---|---|
| `GET /api/resource/viewall`, `/viewall/paginated`, `/find/{id}`, `/find-by-supplier/{id}`, `/find-by-stock/{n}` | all four |
| `POST /api/resource/add` | Admin, Operasional |
| `PUT /api/resource/update/{id}`, `/addToDb/{id}/{stock}`, `/{id}/add-stock`, `/{id}/deduct-stock`, `/add-supplier`, `/update-supplier` | Operasional, Admin |

The asset service, likewise no public routes:

| Route | Roles |
|---|---|
| every `GET /api/asset/**` | all four |
| every `POST`/`PUT`/`DELETE` on `/api/asset/**`, reservations included | Operasional, Admin |
| `GET /api/asset/maintenance/**` | all four |
| `POST /api/asset/maintenance`, `PATCH /api/asset/maintenance/{id}/complete` | Operasional, Admin |

**Maintenance was moved under `/api/asset/`** from the legacy
`/api/maintenance/`, and its writes now carry the same guard as every other
write in the service.

Sitting outside `/api/asset/**` is precisely what let it fall through the
`WebSecurityConfig` per-method rules to `.anyRequest().authenticated()`, so
booking and completing a job were open to every role while the asset writes
beside them were Operasional/Admin. One prefix means one rule, and
`TestEverythingIsUnderAsset` fails if any future route lands outside it.

The role change is enforcement, not new policy: `DetailAssetView.vue` already
gated every maintenance button on `canEditAsset`, which is `Operasional ||
Admin`. The server had simply been trusting the client.

Route changes, all applied to the frontend in the same pass:

| Was | Now |
|---|---|
| `POST /api/maintenance/` (trailing slash load-bearing) | `POST /api/asset/maintenance` |
| `GET /api/maintenance/all` | `GET /api/asset/maintenance/all` |
| `GET /api/maintenance/maintenance-in-progress` | `GET /api/asset/maintenance/in-progress` |
| `PATCH /api/maintenance/{id}/complete` | `PATCH /api/asset/maintenance/{id}/complete` |
| `GET /api/maintenance/{platNomor}` | **removed** — duplicated `GET /api/asset/{platNomor}/maintenance` |

This is a deliberate departure from "route paths still matter" at the top of
this file: nothing is running, the frontend is the only caller, and the two
repos ship together. The cost of this change only goes up from here.

### Error statuses are uniform

Every endpoint in the Go services maps errors the same way:

| Status | Meaning | Message |
|---|---|---|
| **400** | caller mistake — validation, a malformed id or UUID, unparseable JSON | the specific Indonesian message, shown to the user |
| **404** | no resource with that id | `Resource dengan ID {id} tidak ditemukan.` |
| **500** | anything else | a generic `Terjadi kesalahan pada server`; the real error goes to the request log |

`statusFor` and `respondError` in each service's `internal/handler` package are
where this lives, and `resource_handler_test.go` pins it.

This is a **change from Java**, which had no consistent rule at all. The
`ResourceController` wrapped every method in its own catch blocks and they
disagreed with each other — a missing resource was 404 on `find/{id}` but 400 on
`update/{id}`; an unexpected failure was 500 on `viewall/paginated` but 400 on
`viewall`. The same condition also carried three different messages depending on
which service method raised it (`Resource not found`, `Resource tidak
ditermukan` — sic — and `Resource dengan ID x tidak ditemukan.`). In the Go port
"missing resource" is one error type with one message and one status.

The 500 case does **not** echo the underlying error. Java put
`e.getMessage()` straight into the response body, so a failed connection would
have handed the browser the database DSN, credentials included.

Porting another service? Copy this rule rather than the Java controller's catch
blocks. Two error types in the service package carry it: `InvalidError` (400)
and `NotFoundError` (404); anything else is a 500.

### Role storage is flattened

Java modelled roles with JPA JOINED inheritance: an `end_user` table plus
*empty* `Admin`/`Operasional`/`Finance`/`Direksi` child tables keyed by a
discriminator. The Go port collapses that to a **single `end_users` table with a
`user_type` column**. `user_type` keeps the UPPERCASE values because the role
casing map (above) is built around them.

### Fixed: the entire financial ledger had no auth

The finance service's `WebSecurityConfig` declared:

```java
.requestMatchers("/api/lapkeu/**").permitAll()
```

so every route of the financial ledger — read, write and **delete** — was
reachable without a token. `GET /api/lapkeu/all` returned every financial record
to anyone who could reach the service, and `DELETE /api/lapkeu/{id}` removed
entries the same way.

It was load-bearing, which is why it survived: `asset`, `purchase` and `project`
all POST to `/lapkeu/add` **without a bearer token**, unlike every other
inter-service call in the codebase, and `permitAll` was what made those work.

The Go port had already solved that half by accident: those three services call
finance through `httpx.Client`, which forwards the caller's token, so the ledger
now records who triggered each entry. Roles follow the two dashboards that
consume the data — reads are Admin, Finance and Direksi, matching the existing
`/api/finance-report/**` rule; Operasional is deliberately excluded. Writes admit
all four, since they arrive from another service carrying the token of whoever
triggered the flow and Operasional completes purchases and projects. Deletes are
Finance and Admin, because only a Finance refund removes an entry.
`TestLedgerRequiresAToken` is the regression guard.

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

In the resource service:

- **Stock changes are always under a row lock.** Java locked
  (`@Lock(PESSIMISTIC_WRITE)`) for `add-stock` and `deduct-stock` but *not* for
  `addToDb/{id}/{stock}`, the endpoint the purchase service calls, so two
  concurrent purchases of the same item could each read the same starting stock
  and one increment would be lost. `ResourceRepository.UpdateStock` takes the
  lock for all three; the arithmetic is passed in as a function, which is also
  what makes it testable without a database.
- **Supplier links are a set.** The Java `@ElementCollection List<UUID>` was
  maintained by loading the list, checking `contains`, appending, and saving —
  the same lost-update race. The Go `resource_suppliers` table has a composite
  primary key and links are attached with `ON CONFLICT DO NOTHING`.
- `PUT /update/{id}` **rejects a negative stock**. The legacy DTO declares
  `@Min(0)` on it, but that controller never inspects the `BindingResult`, so
  the annotation did nothing and a negative stock was written straight through.
- A **malformed supplier UUID** is rejected with 400 before the query runs. Java
  did this for `find-by-supplier` (via `UUID.fromString`) but not for the
  `add-supplier` / `update-supplier` bodies, where a null or bad id reached
  Hibernate.
- `GET /viewall` is **ordered by id**. Java's `findAll()` had no `ORDER BY`, so
  its row order was whatever Postgres returned.
- The numeric request fields (`resourceStock`, `resourcePrice`, `quantity`) are
  **pointers** in the Go DTOs. Gin's `required` treats a plain `int`'s zero
  value as missing, which would reject a free item or an out-of-stock one;
  on a pointer it only rejects an absent field, which is what `@NotNull` meant.
  `internal/dto/resource_test.go` pins both halves of that.

- **Error statuses are uniform** (400/404/500), where the legacy controller's
  catch blocks disagreed endpoint by endpoint. See "Error statuses are uniform"
  above; the frontend was updated to match.

The frontend's `src/stores/resource.ts` changed with this service: it was
requesting `GET /resource/{id}` (a route neither stack ever served — the real one
is `/resource/find/{id}`), and every `catch` read `error.response.data.message`,
which itself throws on a network error or timeout, where `error.response` is
undefined. Both are fixed, and the store now records the HTTP status alongside
the message so callers can tell a 404 apart from a 500.

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

Eleven shared pieces exist now; use them rather than reinventing per service.

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

**`pkg/page`** is the slice of Spring Data's `Page` JSON the frontend consumes
(`content`, `number`, `size`, `totalElements`, `totalPages`, `first`, `last`,
`numberOfElements`, `empty`). Build one with `page.New(content, number, size,
total)`. Services re-export it from their `dto` package as `dto.PageOf` /
`dto.NewPage`, because several paginated methods take a parameter literally
named `page`, which would shadow the import.

**`pkg/blob`** stores the images the asset and purchase services accept. GCS in
deployment, a local directory when `GCS_BUCKET` is unset — that fallback is what
lets `make run` and the tests work without Google credentials. It deliberately
avoids `cloud.google.com/go/storage`: measured against this module's 60-module
baseline, the official client adds 139 modules and a 47M binary to do three
operations, where `golang.org/x/oauth2` plus `net/http` adds two. Authentication
is still Google's library, so on a GCE VM it reads the instance metadata server
and there is no key file to deploy. `ValidKey` rejects path traversal at every
backend method, and `ContentTypeFor` rejects non-images.

**`pkg/reporting`** holds the calendar arithmetic behind every activity chart
and period summary: what window a named range covers, which granularities that
range can be charted at, and the ordered period labels to plot. It carries no
I/O and no domain knowledge — the caller supplies the counts. purchase and
project chart different things over identical periods, and finance.report will
make a third.

**`httpx.Client.PostForm`** sends multipart/form-data — the stand-in for
Spring's `BodyInserters.fromMultipartData`, used when purchase hands a completed
asset purchase to the asset service. The body streams through an `io.Pipe`, so
forwarding a 10MB photo costs a buffer rather than a copy of the image.

**`pkg/apierr`** holds the two error types every service's handlers branch on:
`Invalid` (400) and `NotFound` (404). Pair it with `httpx.RespondError`, which
also keeps a 500's underlying error out of the response body.

**`pkg/jsontime`** renders dates the way Jackson did once Spring Boot disabled
`WRITE_DATES_AS_TIMESTAMPS`. Use `jsontime.UTC` for fields with no
`@JsonFormat`, `jsontime.Jakarta` for annotated ones.

**`database.Migrate(db, migrations.FS)`** applies a service's pending
migrations at start and logs which ran. Each service owns its database, so each
keeps its own numbering, starting from a `00001_baseline.sql` generated by
running the old `AutoMigrate` against an empty PostgreSQL 16 and dumping the
result — so the first migration is exactly the schema the service already ran
with. All seven baselines were verified to reproduce that dump byte for byte.

To change a schema, add the next numbered `.sql` file beside it; goose runs
what is pending. `pkg/database/migrate_test.go` exercises the runner against a
real database and skips unless `TEST_DATABASE_URL` is set.

**`config.Base.AllowedOrigins()`** returns the CORS origins to hand
`httpx.CORS`. The Java `CorsConfig` in every service also listed the sibling
services' base URLs, but those are server-to-server callers that never send an
`Origin` header, so only `FRONTEND_URL` is in the list.

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

- Refresh tokens, and `jti` + a revocation list so logout is real.
- Reconsider whether `POST /api/profile/add` should stay public.
- Multi-role support (the frontend assumes one role today).
- JWKS endpoint on profile for key rotation.
