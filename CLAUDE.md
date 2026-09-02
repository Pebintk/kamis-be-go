# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is in this repo

KAMIS (a.k.a. "SI KAMIS" / sikamis.com) is an internal management system for PT Karina. This repo holds the **Go rewrite** of its backend.

- `KAMIS-BE-GO/` — the only tracked project. Go monorepo, shared `pkg/`, one deployable per directory under `services/`. **Start with `KAMIS-BE-GO/MIGRATION.md`**: it records the locked decisions, the per-service port status, and the contract quirks the frontend depends on. `KAMIS-BE-GO/README.md` covers layout and commands.
- `KAMIS-BE/` and `KAMIS-FE/` — **gitignored local checkouts of the legacy Java and Vue apps**, kept as a read-only porting reference. They are not part of this repo and will be absent from a fresh clone. The frontend has its own repo at `git@github.com:Pebintk/kamis-fe-migrate.git`.

The rest of this file is orientation for those two reference checkouts, for use while porting.

## Legacy backend (`KAMIS-BE/`, reference only)

Six independent Spring Boot 3.4 / Java 21 microservices, each a standalone Gradle project with its own Postgres database. Group `gpl.karina`, base package `gpl.karina.<service>`. All controllers map under `/api`; in Docker they reach each other at `http://<service>-service:<port>/api`.

| Service | Port | Responsibility |
|---|---|---|
| `profile` | 8080 | Auth + users/roles, clients, suppliers. **Only service that issues JWTs.** |
| `asset` | 8081 | Assets (vehicles, by plate number) + maintenance. Stores images on disk. |
| `finance.report` | 8082 | Financial reports, operational dashboard data. |
| `project` | 8083 | Projects: distribution (`Distribution`) and sales/sell (`Sales`). |
| `purchase` | 8084 | Purchasing of assets and resources. Stores images on disk. |
| `resource` | 8085 | Resource/inventory catalog. |

Every service uses the same package layout (`model/`, `repository/`, `restcontroller/`, `restdto/{request,response}/`, `restservice/`, `security/`), which the Go port mirrors under `internal/` — so ports are near-mechanical.

### Auth model

- **Asymmetric RSA JWT.** `profile` holds both keys and signs with the private key (`JWT_SECRET_KEY`, base64 PKCS8). Every other service is given **only** the public key (`JWT_PUBLIC_KEY`, base64 X509) and validates tokens locally — there is no call back to `profile` to verify a token.
- Roles: Admin, Operasional, Finance, Direksi (plus the `EndUser` base). In `profile` each role is a separate JPA entity related to `EndUser`. Role casing differs by context — see MIGRATION.md.
- Inter-service calls use Spring `WebClient`, forwarding the caller's JWT.

### Configuration

Config comes from environment variables loaded from a per-service `.env` (`spring.config.import: optional:file:.env`); copy each service's `.env.example`. **JPA runs `ddl-auto: update` against PostgreSQL — the schema is auto-managed and there is no migration tool**, so there is no schema history to consult when porting a model.

Build with `./gradlew` inside a service directory. Note CI runs `clean assemble` specifically, not `build`.

## Legacy frontend (`KAMIS-FE/kamis-fe/`, reference only)

Vue 3 `<script setup>` + TypeScript + Vite SPA (Pinia, vue-router, Tailwind v4), feature-foldered by the same six domains under `src/` — `views/`, `interfaces/`, `stores/` (stores own both API calls and state), plus shared `V*`-prefixed components.

- **There is no API gateway.** `config/api.config.ts` gives each backend service its own base URL from a `VITE_API_*_URL` env var and the frontend talks to each microservice directly. Adopting a ported Go service means repointing one of these.
- `router/index.ts` route `meta` drives access control: `requiresAuth` plus a `roles` array, checked by a global guard against `useAuthStore`. Role names here are PascalCase ("Operasional", "Finance"), matching the JWT `role` claim.
- Auth uses `axios` + `jwt-decode`. `src/config/http.ts` attaches the bearer token through a single global request interceptor, installed from `main.ts` — don't add per-call `Authorization` headers.
- **No test runner is configured.** `npm run build` runs `vue-tsc` type-checking as part of the build.

## Deployment (legacy)

GitLab CI in each of `KAMIS-BE/` and `KAMIS-FE/` builds on the `main` / `gcp-deploy` branches: each BE service publishes a Docker image to `gcr.io/<project>`, deployed over SSH using `docker-compose-deploy.yml` onto the `kamis-network` Docker network, where services resolve each other by container name. The Go services are not wired into this yet.
