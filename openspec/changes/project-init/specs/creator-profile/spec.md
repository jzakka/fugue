## ADDED Requirements

### Requirement: Get creator by ID
`GET /api/creators/:id` SHALL return a creator's full profile including their works.

#### Scenario: Valid creator ID
- **WHEN** GET `/api/creators/{valid-uuid}`
- **THEN** response includes creator profile with nickname, bio, roles, contacts, avatar_url, and their works list

#### Scenario: Creator not found
- **WHEN** GET `/api/creators/{non-existent-uuid}`
- **THEN** response status is 404

### Requirement: Update own profile
`PUT /api/creators/me` SHALL update the authenticated creator's profile.

#### Scenario: Update nickname and bio
- **WHEN** PUT `/api/creators/me` with body `{"nickname": "NewName", "bio": "New bio"}`
- **THEN** the creator's nickname and bio are updated, response includes the updated profile

#### Scenario: Nickname is required
- **WHEN** PUT `/api/creators/me` with empty nickname
- **THEN** response status is 400 with validation error

#### Scenario: Roles must be non-empty
- **WHEN** PUT `/api/creators/me` with empty roles array
- **THEN** response status is 400 with validation error

#### Scenario: Contacts must have at least one entry
- **WHEN** PUT `/api/creators/me` with empty contacts
- **THEN** response status is 400 with validation error

### Requirement: List creators with role filter
`GET /api/creators` SHALL return a paginated list of creators, optionally filtered by role.

#### Scenario: Filter by role
- **WHEN** GET `/api/creators?roles=music&page=1&limit=20`
- **THEN** response includes only creators whose roles contain "music", with pagination metadata

#### Scenario: No filter returns all
- **WHEN** GET `/api/creators?page=1&limit=20`
- **THEN** response includes all creators, paginated

#### Scenario: Default pagination
- **WHEN** GET `/api/creators` without page/limit params
- **THEN** defaults to page=1, limit=20
