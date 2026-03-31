## ADDED Requirements

### Requirement: Fetch OG metadata from URL
The API SHALL accept a URL via `POST /api/og/fetch` and return OG metadata: title, description, image URL, and site name.

#### Scenario: URL with OG tags returns metadata
- **WHEN** POST `/api/og/fetch` with body `{"url": "https://soundcloud.com/artist/track"}`
- **THEN** response includes `{"data": {"title": "...", "description": "...", "image": "...", "site_name": "SoundCloud"}}`

#### Scenario: URL without OG tags returns partial data
- **WHEN** POST `/api/og/fetch` with a URL that has no OG meta tags
- **THEN** response includes whatever is available (title from `<title>` tag, others null)

#### Scenario: Invalid URL returns error
- **WHEN** POST `/api/og/fetch` with body `{"url": "not-a-url"}`
- **THEN** response status is 400 with error message

### Requirement: OG fetch times out at 5 seconds
The HTTP request to fetch OG metadata SHALL timeout after 5 seconds.

#### Scenario: Slow URL times out
- **WHEN** the target URL takes longer than 5 seconds to respond
- **THEN** response status is 422 with error indicating timeout

### Requirement: OG results are cached in Redis
Successful OG fetch results SHALL be cached in Redis with a 24-hour TTL keyed by URL.

#### Scenario: Cached URL returns instantly
- **WHEN** POST `/api/og/fetch` is called for a URL that was fetched within the last 24 hours
- **THEN** the cached result is returned without making an HTTP request to the target

#### Scenario: Cache expires after 24 hours
- **WHEN** 24 hours have passed since the last fetch for a URL
- **THEN** the next request fetches fresh data from the target URL

### Requirement: Auto-detect field from URL domain
The OG fetch response SHALL include a `detected_field` based on the URL domain.

#### Scenario: SoundCloud URL detected as music
- **WHEN** URL domain is soundcloud.com
- **THEN** `detected_field` is "music"

#### Scenario: pixiv URL detected as illustration
- **WHEN** URL domain is pixiv.net
- **THEN** `detected_field` is "illustration"

#### Scenario: YouTube URL detected as video
- **WHEN** URL domain is youtube.com or youtu.be
- **THEN** `detected_field` is "video"

#### Scenario: Unknown domain returns null field
- **WHEN** URL domain is not in the known list
- **THEN** `detected_field` is null
