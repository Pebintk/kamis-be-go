# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

KAMIS (a.k.a. "SI KAMIS" / sikamis.com) is an internal management system for PT Karina. The repo is split into two top-level projects that are deployed separately:

- `KAMIS-BE/` — Java 21 / Spring Boot 3.4 backend, organized as **six independent microservices**, each its own Gradle project.
- `KAMIS-FE/kamis-fe/` — Vue 3 + TypeScript + Vite single-page app (Pinia, vue-router, Tailwind v4).

There is no root build file or git repo at the top level; each microservice and the frontend are built independently.

## Backend (`KAMIS-BE/`)

### The six services

Each directory is a standalone Spring Boot app with its own `build.gradle`, `gradlew`, `Dockerfile`, and Postgres database. Group is `gpl.karina`, base package `gpl.karina.<service>`.

| Service | Port | Package | Responsibility |
|---|---|---|---|
| `profile` | 8080 | `gpl.karina.profile` | Auth + users/roles, clients, suppliers. **Only service that issues JWTs.** |
| `asset` | 8081 | `gpl.karina.asset` | Assets (vehicles, by plate number) + maintenance. Stores images on disk. |
| `finance.report` | 8082 | `gpl.karina.financereport` | Financial reports, operational dashboard data. |
| `project` | 8083 | `gpl.karina.project` | Projects: distribution (`Distribution`) and sales/sell (`Sales`). |
| `purchase` | 8084 | `gpl.karina.purchase` | Purchasing of assets and resources. Stores images on disk. |
| `resource` | 8085 | `gpl.karina.resource` | Resource/inventory catalog. |

All controllers are mapped under `/api` (e.g. `@RequestMapping("/api/asset")`). In Docker, services reach each other via `http://<service>-service:<port>/api` (see `docker-compose-deploy.yml`).

### Per-service layout (consistent across all six)

```
src/main/java/gpl/karina/<service>/
  config/         WebClient / app config
  model/          JPA entities
  repository/     Spring Data JPA repositories
  restcontroller/ @RestController endpoints (under /api)
  restdto/request, restdto/response   request & response DTOs
  restservice/    business logic (service layer)
  security/       WebSecurityConfig + jwt/ (JwtTokenFilter, JwtUtils) + service/
```

### Auth model (important)

- **Asymmetric RSA JWT.** `profile` holds both keys: signs tokens with the **private** key (`JWT_SECRET_KEY`, base64 PKCS8) and exposes login. Every other service is given **only the public key** (`JWT_PUBLIC_KEY`, base64 X509) and validates tokens locally — there is no call back to `profile` to verify a token.
- Each service has its own `JwtTokenFilter` + `JwtUtils` under `security/jwt/`. The public-key `JwtUtils` extracts `subject` (username) and a `role` claim.
- Roles: **Admin, Operasional, Finance, Direksi** (plus the `EndUser` base). In `profile`, each role is a separate JPA entity (`Admin.java`, `Operasional.java`, etc.) subclassing/related to `EndUser`.
- Inter-service calls use Spring `WebClient` (webflux), forwarding the caller's JWT.

### Configuration

- Config is read from environment variables, loaded from a per-service `.env` file (`spring.config.import: optional:file:.env`). Copy `.env.example` in each service. Key vars: `DATABASE_URL_<SERVICE>`, `DATABASE_USERNAME/PASSWORD`, `JWT_PUBLIC_KEY` (all services), `JWT_SECRET_KEY` + `JWT_EXPIRATION_MS` + `ADMIN_*` (profile only), and the `*_URL` service-discovery vars.
- `application.yml` lives in `src/main/resources/`. JPA uses `ddl-auto: update` against PostgreSQL — schema is auto-managed, no migration tool.

### Common commands (run inside a service directory, e.g. `KAMIS-BE/profile/`)

```bash
./gradlew bootRun              # run the service locally
./gradlew clean assemble       # build jar (build/libs/<service>-0.0.1-SNAPSHOT.jar) — what CI runs
./gradlew build                # build + run tests
./gradlew test                 # run all tests (JUnit 5 + Mockito)
./gradlew test --tests 'gpl.karina.profile.SomeTest'          # single test class
./gradlew test --tests 'gpl.karina.profile.SomeTest.method'   # single test method
```

## Frontend (`KAMIS-FE/kamis-fe/`)

Vue 3 `<script setup>` SPA. Source is feature-foldered by the same six domains under `src/`:

- `views/<domain>/` — page components (asset, finance.report, profile, project, purchase, resource, errorpage).
- `interfaces/<domain>/` — TypeScript types per domain.
- `stores/` — Pinia stores (`auth.ts`, plus one per domain: `asset.ts`, `purchase.ts`, `project.ts`, `account.ts`, `client.ts`, `supplier.ts`, `financereport.ts`, etc.). Stores own API calls + state.
- `components/` — shared UI (the `V*`-prefixed components are the reusable form/widget kit).
- `config/api.config.ts` — central API config. Each backend service has its **own** base URL from a `VITE_API_*_URL` env var (`VITE_API_PROFILE_URL`, `VITE_API_ASSET_URL`, …). There is no API gateway; the frontend talks to each microservice directly. `API_ENDPOINTS` holds endpoint path builders.
- `router/index.ts` — all routes. Route `meta` drives access control: `requiresAuth: boolean` and `roles: string[]` (e.g. `['Operasional']`, `['Finance']`). The global guard checks these against `useAuthStore`. Note role names here are the human-readable form ("Operasional", "Finance", "Admin", "Direksi"), matching the JWT role claim.

Auth: `axios` for HTTP, `jwt-decode` to read the token; the auth store holds the token + decoded role.

### Common commands (run inside `KAMIS-FE/kamis-fe/`)

```bash
npm install
npm run dev          # Vite dev server
npm run build        # type-check (vue-tsc) + production build
npm run type-check   # vue-tsc --build only
npm run lint         # eslint --fix
npm run format       # prettier --write src/
```

There is no test runner configured for the frontend.

## Deployment

GitLab CI (`.gitlab-ci.yml` in both `KAMIS-BE/` and `KAMIS-FE/`) builds on the `main` / `gcp-deploy` branches: each BE service is built and published as a Docker image to `gcr.io/<project>`, then deployed via SSH using `docker-compose-deploy.yml`. The frontend builds to a Docker image served behind the public domain. Backend services run on the `kamis-network` Docker network and resolve each other by container name (`<service>-service`).
