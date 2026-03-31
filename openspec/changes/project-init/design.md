## Context

Fugue is a greenfield project with detailed PRD and architecture docs but no code. The architecture specifies Go + Chi backend, Next.js 15 frontend, PostgreSQL 16 with sqlc, and Redis. The monorepo structure (`apps/api/`, `apps/web/`) and all API endpoints are already defined. This design covers how to scaffold everything from zero to a locally-runnable state.

## Goals / Non-Goals

**Goals:**
- Runnable Go API with all MVP endpoints wired up
- Runnable Next.js frontend with basic page structure
- Local dev environment via docker-compose (PostgreSQL + Redis)
- DB migrations and sqlc-generated code for all CRUD operations
- Clean project structure that matches the architecture doc

**Non-Goals:**
- Terraform/infrastructure setup (separate change)
- Helm charts and K8s manifests (separate change)
- GitHub Actions CI/CD pipelines (separate change)
- Observability stack (Prometheus, Grafana, Loki, Tempo)
- OAuth provider integration (will stub auth, real OAuth is a follow-up)
- Production-ready error handling, rate limiting, graceful shutdown
- Frontend UI polish, styling system, or component library selection

## Decisions

### 1. Go project structure: standard layout with `cmd/` and `internal/`

Follows Go community conventions. `cmd/server/main.go` is the entrypoint. `internal/` contains `config/`, `handler/`, `service/`, `repository/`, `model/`, `middleware/`.

**Alternative considered**: flat package structure. Rejected because the architecture doc already specifies the layered approach and it scales better as the codebase grows.

### 2. Database driver: pgx v5

pgx is the most performant pure-Go PostgreSQL driver and is the default for sqlc. pgxpool provides connection pooling out of the box.

**Alternative considered**: lib/pq. Rejected because it's in maintenance mode and pgx has better performance and feature support.

### 3. Migration tool: golang-migrate

File-based migrations (`db/migrations/NNNN_name.up.sql` / `.down.sql`). Simple, no ORM dependency, works well with sqlc's SQL-first approach.

**Alternative considered**: goose. Both are viable, but golang-migrate has broader adoption and simpler CLI.

### 4. sqlc configuration

sqlc will generate Go code from SQL queries in `db/queries/`. Output goes to `internal/repository/sqlc/`. We'll use pgx/v5 as the SQL engine.

### 5. Next.js: App Router with `src/` directory

Using `src/app/` layout for clearer separation. TypeScript strict mode. Pages for: home, explore (works + creators), work detail, creator profile, recommend, login.

### 6. API response format

Standard JSON envelope:
```json
{
  "data": { ... },
  "error": null,
  "meta": { "page": 1, "limit": 20, "total": 100 }
}
```

Pagination via query params `?page=1&limit=20`. Default limit 20, max 100.

### 7. Auth stub strategy

MVP stubs auth with a middleware that reads a `X-Creator-ID` header in dev mode. This unblocks all authenticated endpoints without requiring OAuth provider setup. Real OAuth (Twitter/Discord) will be a separate change.

### 8. OG fetch implementation

Server-side HTTP fetch of the target URL, parse `<meta property="og:*">` tags from HTML head. Cache results in Redis with 24h TTL. Timeout at 5 seconds. Domain-based field auto-detection (soundcloud.com -> music, pixiv.net -> illustration, etc.).

## Risks / Trade-offs

- **[Auth stub is not real auth]** -> Acceptable for local dev. Real OAuth is a tracked follow-up. The middleware interface is designed so swapping in real auth requires no handler changes.
- **[OG fetch can be slow/blocked]** -> 5s timeout + Redis cache mitigates this. Some sites block server-side fetches. We'll handle failures gracefully (return partial data).
- **[sqlc code generation requires manual step]** -> Developers must run `sqlc generate` after changing queries. We'll document this clearly. CI will verify generated code is up to date.
- **[No frontend API client codegen]** -> Manual TypeScript fetch wrappers for now. OpenAPI codegen is a future optimization.
