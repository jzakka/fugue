## Why

Fugue has PRD, architecture docs, and a CLAUDE.md spec but zero runnable code. The `apps/`, `helm/`, `terraform/`, and `docker-compose.yml` described in the architecture doc don't exist yet. We need to bootstrap the codebase so development can begin.

## What Changes

- Scaffold Go backend (`apps/api/`) with Chi router, sqlc setup, and standard project layout (`cmd/`, `internal/`)
- Scaffold Next.js 15 frontend (`apps/web/`) with App Router and TypeScript
- Create PostgreSQL schema migrations for `creators` and `works` tables
- Create sqlc queries and generate Go code for all CRUD operations
- Add `docker-compose.yml` for local dev (PostgreSQL 16 + Redis)
- Wire up API endpoints: auth stubs, creators CRUD, works CRUD, recommend, OG fetch
- Add Dockerfiles for both api and web (multi-stage builds)

## Capabilities

### New Capabilities
- `go-api-scaffold`: Go backend with Chi router, config, health check, and middleware (CORS, logging, auth placeholder)
- `db-schema`: PostgreSQL migrations for creators and works tables, sqlc query definitions and code generation
- `og-fetch`: OG metadata fetching service (URL -> title, thumbnail, description)
- `creator-profile`: Creator CRUD endpoints (GET, PUT /me, list with role filter)
- `work-submission`: Work CRUD endpoints (POST, GET, DELETE, list with field/tag filter)
- `recommendation`: Tag-based work recommendation by project type
- `nextjs-scaffold`: Next.js 15 App Router frontend with TypeScript, basic pages and API client
- `local-dev`: docker-compose for PostgreSQL + Redis, Dockerfiles for api and web

### Modified Capabilities

(none - greenfield project)

## Impact

- **New files**: ~50-70 files across `apps/api/`, `apps/web/`, `docker-compose.yml`
- **Dependencies**: Go modules (chi, pgx, sqlc), npm packages (next, react, typescript)
- **APIs**: All endpoints from the spec become available (stubs for auth, full impl for works/creators/recommend/og)
- **DB**: PostgreSQL schema with GIN indexes for tags and roles
- **Dev workflow**: `docker-compose up` + `go run` + `npm run dev` becomes functional
