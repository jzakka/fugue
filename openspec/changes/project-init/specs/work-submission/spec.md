## ADDED Requirements

### Requirement: Submit a work
`POST /api/works` SHALL create a new work for the authenticated creator.

#### Scenario: Valid work submission
- **WHEN** POST `/api/works` with body `{"url": "https://soundcloud.com/x/y", "title": "My Track", "field": "music", "tags": ["electronic", "dark"]}`
- **THEN** the work is created, response status is 201 with the created work

#### Scenario: URL is required
- **WHEN** POST `/api/works` without url
- **THEN** response status is 400

#### Scenario: Tags must have 1-5 items
- **WHEN** POST `/api/works` with empty tags or more than 5 tags
- **THEN** response status is 400 with validation error

#### Scenario: Field must be valid
- **WHEN** POST `/api/works` with field="invalid"
- **THEN** response status is 400 with error listing valid fields (music, illustration, video, 3d, sound)

### Requirement: Get work by ID
`GET /api/works/:id` SHALL return the work with its creator info.

#### Scenario: Valid work ID
- **WHEN** GET `/api/works/{valid-uuid}`
- **THEN** response includes work details and creator summary (id, nickname, avatar_url)

#### Scenario: Work not found
- **WHEN** GET `/api/works/{non-existent-uuid}`
- **THEN** response status is 404

### Requirement: Delete own work
`DELETE /api/works/:id` SHALL delete a work if the authenticated creator owns it.

#### Scenario: Owner deletes work
- **WHEN** authenticated creator sends DELETE `/api/works/{own-work-id}`
- **THEN** work is deleted, response status is 204

#### Scenario: Non-owner cannot delete
- **WHEN** authenticated creator sends DELETE `/api/works/{other-creators-work-id}`
- **THEN** response status is 403

### Requirement: List works with filters
`GET /api/works` SHALL return a paginated list of works with optional filters.

#### Scenario: Filter by field
- **WHEN** GET `/api/works?field=music&page=1&limit=20`
- **THEN** response includes only works with field="music"

#### Scenario: Filter by tags
- **WHEN** GET `/api/works?tags=electronic,dark&page=1&limit=20`
- **THEN** response includes works whose tags overlap with ["electronic", "dark"]

#### Scenario: Combined filters
- **WHEN** GET `/api/works?field=music&tags=electronic&page=1&limit=20`
- **THEN** response includes works matching both field and tag filters

#### Scenario: Sort by latest
- **WHEN** GET `/api/works?page=1&limit=20`
- **THEN** works are sorted by created_at DESC
