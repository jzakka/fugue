## ADDED Requirements

### Requirement: Next.js 15 App Router project
The frontend SHALL be a Next.js 15 project using App Router with TypeScript in `apps/web/`. The project SHALL use the `src/` directory layout.

#### Scenario: Dev server starts
- **WHEN** `npm run dev` is run from `apps/web/`
- **THEN** the Next.js dev server starts on port 3000

### Requirement: Page routes match user flows
The frontend SHALL have the following routes:
- `/` - Home page (landing with hero + recent works)
- `/explore` - Explore works and creators
- `/works/[id]` - Work detail page
- `/creators/[id]` - Creator profile page
- `/recommend` - Recommendation flow page
- `/login` - Login page

#### Scenario: All routes render without errors
- **WHEN** each route is visited in the browser
- **THEN** the page renders without JavaScript errors

### Requirement: API client module
The frontend SHALL have a typed API client module that wraps fetch calls to the Go backend. Base URL configurable via `NEXT_PUBLIC_API_URL` env var (default `http://localhost:8080`).

#### Scenario: API client makes typed requests
- **WHEN** `api.works.list({field: "music"})` is called
- **THEN** it sends GET to `{API_URL}/api/works?field=music` and returns typed response

### Requirement: Environment configuration
The frontend SHALL read `NEXT_PUBLIC_API_URL` from environment variables for API base URL.

#### Scenario: Default API URL
- **WHEN** `NEXT_PUBLIC_API_URL` is not set
- **THEN** the API client defaults to `http://localhost:8080`
