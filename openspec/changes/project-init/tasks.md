## 1. Local Dev Environment

- [ ] 1.1 Create `docker-compose.yml` with PostgreSQL 16 and Redis services (named volumes, default creds fugue/fugue)
- [ ] 1.2 Verify `docker-compose up -d` starts both services and they're reachable

## 2. Go API Scaffold

- [ ] 2.1 Initialize Go module (`apps/api/go.mod`) with chi, pgx/v5, and redis dependencies
- [ ] 2.2 Create `cmd/server/main.go` entrypoint with config loading, DB pool init, and HTTP server startup
- [ ] 2.3 Create `internal/config/` package to load env vars (DATABASE_URL, REDIS_URL, PORT, ENV, CORS_ORIGIN)
- [ ] 2.4 Create `internal/middleware/` with CORS, request logging, and auth stub (X-Creator-ID header)
- [ ] 2.5 Create `internal/model/` with Go structs for Creator, Work, and API response envelope
- [ ] 2.6 Wire up Chi router with `/api/health` endpoint and route groups for all resources

## 3. Database Schema & sqlc

- [ ] 3.1 Create `db/migrations/000001_init.up.sql` with creators table, works table, and all indexes
- [ ] 3.2 Create `db/migrations/000001_init.down.sql` with reverse migration
- [ ] 3.3 Create `db/sqlc.yaml` configuration (pgx/v5 engine, output to internal/repository/sqlc/)
- [ ] 3.4 Create `db/queries/creators.sql` with CreateCreator, GetCreatorByID, UpdateCreator, ListCreators queries
- [ ] 3.5 Create `db/queries/works.sql` with CreateWork, GetWorkByID, DeleteWork, ListWorks, ListWorksByCreator, GetRecommendedWorks queries
- [ ] 3.6 Run `sqlc generate` and verify generated code compiles

## 4. API Handlers - Creator Profile

- [ ] 4.1 Create `internal/handler/creator.go` with GET /:id, PUT /me, GET / (list) handlers
- [ ] 4.2 Create `internal/service/creator.go` with business logic layer
- [ ] 4.3 Add input validation (nickname required, roles non-empty, contacts non-empty)
- [ ] 4.4 Wire creator routes into the Chi router

## 5. API Handlers - Work Submission

- [ ] 5.1 Create `internal/handler/work.go` with POST /, GET /:id, DELETE /:id, GET / (list) handlers
- [ ] 5.2 Create `internal/service/work.go` with business logic layer
- [ ] 5.3 Add input validation (url required, field must be valid enum, tags 1-5 items)
- [ ] 5.4 Add ownership check on DELETE (only creator who submitted can delete)
- [ ] 5.5 Wire work routes into the Chi router

## 6. API Handlers - OG Fetch

- [ ] 6.1 Create `internal/service/ogfetch.go` with HTML fetch, meta tag parsing, 5s timeout
- [ ] 6.2 Add Redis caching layer (24h TTL, keyed by URL)
- [ ] 6.3 Add domain-based field auto-detection (soundcloud->music, pixiv->illustration, youtube->video, twitter->video)
- [ ] 6.4 Create `internal/handler/og.go` with POST /api/og/fetch handler
- [ ] 6.5 Wire OG route into the Chi router

## 7. API Handlers - Recommendation

- [ ] 7.1 Create `internal/service/recommend.go` with project type to fields mapping
- [ ] 7.2 Implement recommendation query (filter by fields, tag overlap, exclude own works, order by match count)
- [ ] 7.3 Create `internal/handler/recommend.go` with POST /api/recommend handler
- [ ] 7.4 Add validation (work_id exists, valid project_type, custom requires fields)
- [ ] 7.5 Wire recommend route into the Chi router

## 8. Next.js Frontend Scaffold

- [ ] 8.1 Initialize Next.js 15 project with App Router, TypeScript, src/ directory (`apps/web/`)
- [ ] 8.2 Create API client module (`src/lib/api.ts`) with typed fetch wrappers for all endpoints
- [ ] 8.3 Create page routes: `/`, `/explore`, `/works/[id]`, `/creators/[id]`, `/recommend`, `/login`
- [ ] 8.4 Add basic layout with navigation header
- [ ] 8.5 Configure `NEXT_PUBLIC_API_URL` env var with default to localhost:8080

## 9. Dockerfiles

- [ ] 9.1 Create `apps/api/Dockerfile` with multi-stage build (Go build -> distroless/static)
- [ ] 9.2 Create `apps/web/Dockerfile` with multi-stage build (npm install -> next build -> standalone)
- [ ] 9.3 Verify both images build successfully
