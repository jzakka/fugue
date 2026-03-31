## ADDED Requirements

### Requirement: Recommend works by project type
`POST /api/recommend` SHALL return works that match the given project type's required fields and share tags with the source work.

#### Scenario: MV project type returns illustration and video works
- **WHEN** POST `/api/recommend` with `{"work_id": "{uuid}", "project_type": "mv"}`
- **THEN** response includes works with field in ["illustration", "video"] that share tags with the source work, ordered by tag match count DESC then created_at DESC

#### Scenario: Game project type returns multiple fields
- **WHEN** POST `/api/recommend` with `{"work_id": "{uuid}", "project_type": "game"}`
- **THEN** response includes works with field in ["illustration", "music", "sound", "3d"] with overlapping tags

#### Scenario: Custom project type with user-selected fields
- **WHEN** POST `/api/recommend` with `{"work_id": "{uuid}", "project_type": "custom", "fields": ["music", "sound"]}`
- **THEN** response includes works in the specified fields with overlapping tags

### Requirement: Project type to fields mapping
The API SHALL map project types to required fields:
- mv -> illustration, video
- game -> illustration, music, sound, 3d
- album_artwork -> illustration
- animation -> illustration, music, voice
- voice_drama -> voice, music, sound
- custom -> user-provided fields array

#### Scenario: Invalid project type
- **WHEN** POST `/api/recommend` with `{"work_id": "{uuid}", "project_type": "invalid"}`
- **THEN** response status is 400 with error listing valid project types

#### Scenario: Custom type without fields
- **WHEN** POST `/api/recommend` with `{"work_id": "{uuid}", "project_type": "custom"}` without fields
- **THEN** response status is 400 with error requiring fields for custom type

### Requirement: Source work must exist
The recommendation SHALL validate that the given work_id exists and belongs to the authenticated creator.

#### Scenario: Non-existent work_id
- **WHEN** POST `/api/recommend` with a work_id that doesn't exist
- **THEN** response status is 404

### Requirement: Recommendations exclude own works
The recommendation results SHALL NOT include works by the requesting creator.

#### Scenario: Own works are excluded
- **WHEN** creator A requests recommendations based on their work
- **THEN** none of creator A's other works appear in results
