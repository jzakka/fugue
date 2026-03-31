## ADDED Requirements

### Requirement: docker-compose provides PostgreSQL and Redis
`docker-compose.yml` SHALL define services for PostgreSQL 16 and Redis for local development.

#### Scenario: Services start with docker-compose up
- **WHEN** `docker-compose up -d` is run from the project root
- **THEN** PostgreSQL is available on localhost:5432 and Redis on localhost:6379

#### Scenario: PostgreSQL has default database
- **WHEN** PostgreSQL container starts
- **THEN** a database named `fugue` exists with user `fugue` and password `fugue`

### Requirement: API Dockerfile uses multi-stage build
The `apps/api/Dockerfile` SHALL use a multi-stage build: build stage with Go toolchain, final stage with distroless/static or alpine for minimal image size (~20MB target).

#### Scenario: Docker image builds successfully
- **WHEN** `docker build -t fugue-api .` is run from `apps/api/`
- **THEN** the image builds without errors

#### Scenario: Image runs the server
- **WHEN** the built image is run with required env vars
- **THEN** the API server starts and responds to health checks

### Requirement: Web Dockerfile uses multi-stage build
The `apps/web/Dockerfile` SHALL use a multi-stage build: install deps, build Next.js, final stage with Node.js runtime and standalone output.

#### Scenario: Docker image builds successfully
- **WHEN** `docker build -t fugue-web .` is run from `apps/web/`
- **THEN** the image builds without errors

### Requirement: Volume persistence for local dev data
docker-compose SHALL use named volumes for PostgreSQL data so data persists across container restarts.

#### Scenario: Data survives restart
- **WHEN** `docker-compose down` then `docker-compose up -d` is run
- **THEN** previously inserted data is still in PostgreSQL
