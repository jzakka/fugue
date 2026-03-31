## ADDED Requirements

### Requirement: Go project structure follows standard layout
The project SHALL have `apps/api/cmd/server/main.go` as the entrypoint. The `apps/api/internal/` directory SHALL contain packages: `config`, `handler`, `service`, `repository`, `model`, and `middleware`.

#### Scenario: Project compiles and runs
- **WHEN** developer runs `go run cmd/server/main.go` from `apps/api/`
- **THEN** the HTTP server starts on the configured port (default 8080)

### Requirement: Chi router serves all API endpoints
The API SHALL use Chi router to serve all endpoints under the `/api/` prefix. Routes SHALL be organized by resource group.

#### Scenario: Health check endpoint
- **WHEN** GET request is sent to `/api/health`
- **THEN** response status is 200 with `{"status": "ok"}`

#### Scenario: Unknown route returns 404
- **WHEN** GET request is sent to `/api/nonexistent`
- **THEN** response status is 404

### Requirement: Configuration loaded from environment variables
The API SHALL read configuration from environment variables with sensible defaults for local development. Required config: `DATABASE_URL`, `REDIS_URL`, `PORT`, `ENV` (dev/prod).

#### Scenario: Default config in dev mode
- **WHEN** the server starts without `PORT` set
- **THEN** it defaults to port 8080

#### Scenario: Database URL is required
- **WHEN** the server starts without `DATABASE_URL` set
- **THEN** it SHALL fail with a clear error message

### Requirement: CORS middleware allows frontend origin
The API SHALL include CORS middleware that allows requests from the frontend origin (configurable, defaults to `http://localhost:3000`).

#### Scenario: Preflight request from frontend
- **WHEN** an OPTIONS request with `Origin: http://localhost:3000` is received
- **THEN** response includes `Access-Control-Allow-Origin: http://localhost:3000`

### Requirement: Request logging middleware
The API SHALL log every request with method, path, status code, and duration.

#### Scenario: Successful request is logged
- **WHEN** a GET request to `/api/health` returns 200
- **THEN** a structured log entry includes method=GET, path=/api/health, status=200, duration_ms
