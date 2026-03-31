## ADDED Requirements

### Requirement: Creators table schema
The database SHALL have a `creators` table with columns: `id` (UUID, PK, auto-generated), `nickname` (VARCHAR(50), NOT NULL), `bio` (VARCHAR(200)), `roles` (TEXT[], NOT NULL), `contacts` (JSONB, NOT NULL), `avatar_url` (VARCHAR(500)), `created_at` (TIMESTAMPTZ, default now()), `updated_at` (TIMESTAMPTZ, default now()).

#### Scenario: Creator row is created
- **WHEN** a row is inserted with nickname="TestUser", roles={"music"}, contacts={"twitter":"@test"}
- **THEN** the row is stored with auto-generated UUID and timestamps

### Requirement: Works table schema
The database SHALL have a `works` table with columns: `id` (UUID, PK, auto-generated), `creator_id` (UUID, FK to creators.id, NOT NULL), `url` (VARCHAR(1000), NOT NULL), `title` (VARCHAR(200), NOT NULL), `description` (VARCHAR(500)), `field` (VARCHAR(50), NOT NULL), `tags` (TEXT[], NOT NULL), `og_image` (VARCHAR(1000)), `og_data` (JSONB), `created_at` (TIMESTAMPTZ, default now()).

#### Scenario: Work row references valid creator
- **WHEN** a work is inserted with a valid creator_id
- **THEN** the row is stored successfully

#### Scenario: Work with invalid creator_id is rejected
- **WHEN** a work is inserted with a non-existent creator_id
- **THEN** the database rejects it with a foreign key violation

### Requirement: GIN indexes for array columns
The database SHALL have GIN indexes on `works.tags` and `creators.roles` for efficient array overlap queries.

#### Scenario: Tag overlap query uses GIN index
- **WHEN** a query filters works by `tags && ARRAY['electronic', 'dark']`
- **THEN** the query plan uses the GIN index on tags

### Requirement: Migration files are versioned
Migrations SHALL use numbered files (`000001_init.up.sql`, `000001_init.down.sql`) managed by golang-migrate. The down migration SHALL cleanly reverse the up migration.

#### Scenario: Up migration creates tables
- **WHEN** `migrate up` is run on an empty database
- **THEN** both `creators` and `works` tables exist with all indexes

#### Scenario: Down migration drops tables
- **WHEN** `migrate down` is run after up
- **THEN** both tables and indexes are removed

### Requirement: sqlc generates Go code from queries
sqlc SHALL be configured to read queries from `db/queries/` and generate Go code to `internal/repository/sqlc/`. The generated code SHALL use pgx/v5.

#### Scenario: sqlc generate succeeds
- **WHEN** `sqlc generate` is run from `apps/api/`
- **THEN** Go files are generated in `internal/repository/sqlc/` without errors

### Requirement: CRUD queries for creators
sqlc queries SHALL include: CreateCreator, GetCreatorByID, UpdateCreator, ListCreators (with role filter, pagination).

#### Scenario: List creators filtered by role
- **WHEN** ListCreators is called with role="music" and limit=20
- **THEN** it returns creators whose roles array contains "music", ordered by created_at DESC

### Requirement: CRUD queries for works
sqlc queries SHALL include: CreateWork, GetWorkByID, DeleteWork, ListWorks (with field filter, tag filter, pagination), ListWorksByCreator, GetRecommendedWorks.

#### Scenario: List works filtered by field and tags
- **WHEN** ListWorks is called with field="illustration" and tags=["fantasy", "dark"]
- **THEN** it returns works with field="illustration" whose tags overlap with the given tags

#### Scenario: Get recommended works by project type fields and tags
- **WHEN** GetRecommendedWorks is called with fields=["illustration", "video"] and tags=["electronic", "dark"]
- **THEN** it returns works in those fields with overlapping tags, ordered by tag match count DESC then created_at DESC
