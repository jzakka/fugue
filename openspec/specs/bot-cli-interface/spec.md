## ADDED Requirements

### Requirement: CLI root command provides usage guidance
The root command (executed without subcommands) SHALL display help information including available subcommands and basic usage.

#### Scenario: User runs fuguebot without arguments
- **WHEN** user executes the bot binary without any subcommands or arguments
- **THEN** system displays help text listing available subcommands (pioneer, harvester) and usage examples

### Requirement: Site name resolution supports both short names and domains
The system SHALL accept both short source names (e.g., "unsplash", "fma") and full domain names (e.g., "unsplash.com") as site identifiers, resolving short names to domains via a registry.

#### Scenario: User provides short source name
- **WHEN** user executes a command with a short source name (e.g., "unsplash")
- **THEN** system resolves it to the corresponding domain (e.g., "unsplash.com") using the source registry

#### Scenario: User provides full domain name
- **WHEN** user executes a command with a full domain name (e.g., "unsplash.com")
- **THEN** system uses the domain directly without resolution

#### Scenario: User provides unknown source name
- **WHEN** user executes a command with a source name not in the registry and not a valid domain format
- **THEN** system displays an error message listing available source names and exits with error code

### Requirement: Pioneer subcommand executes site exploration
The pioneer subcommand SHALL initialize and run the Pioneer crawler for a specified site.

#### Scenario: User runs pioneer with valid site name
- **WHEN** user executes `pioneer <site>` with a site name (short or domain) that resolves to a domain existing in the database
- **THEN** system resolves the site name to domain, initializes database and storage connections, creates a Pioneer instance with AI client and necessary repositories, and executes the pioneer crawl for that site

#### Scenario: User runs pioneer with non-existent site
- **WHEN** user executes `pioneer <site>` with a site name that resolves to a domain not found in the database
- **THEN** system displays an error message indicating the site domain was not found in the database and exits with non-zero code

#### Scenario: User runs pioneer without site argument
- **WHEN** user executes `pioneer` without providing a site name
- **THEN** system displays usage help for the pioneer command and exits with error code

### Requirement: Harvester subcommand executes content extraction
The harvester subcommand SHALL initialize and run the Harvester crawler for a specified site.

#### Scenario: User runs harvester with valid site name
- **WHEN** user executes `harvester <site>` with a site name (short or domain) that resolves to a domain existing in the database
- **THEN** system resolves the site name to domain, initializes database and storage connections, creates a Harvester instance with script executor and pipeline, and executes the harvester crawl for that site

#### Scenario: User runs harvester with non-existent site
- **WHEN** user executes `harvester <site>` with a site name that resolves to a domain not found in the database
- **THEN** system displays an error message indicating the site domain was not found in the database and exits with non-zero code

#### Scenario: User runs harvester without site argument
- **WHEN** user executes `harvester` without providing a site name
- **THEN** system displays usage help for the harvester command and exits with error code

### Requirement: Makefile provides convenient shortcuts
The Makefile SHALL provide `pioneer` and `harvester` targets that invoke the corresponding CLI commands.

#### Scenario: User runs make pioneer with SITE variable
- **WHEN** user executes `make pioneer SITE=<site>`
- **THEN** system runs `go run cmd/bot/main.go pioneer <site>`

#### Scenario: User runs make pioneer without SITE variable
- **WHEN** user executes `make pioneer` without providing SITE variable
- **THEN** system displays usage message indicating SITE variable is required and exits with error

#### Scenario: User runs make harvester with SITE variable
- **WHEN** user executes `make harvester SITE=<site>`
- **THEN** system runs `go run cmd/bot/main.go harvester <site>`

#### Scenario: User runs make harvester without SITE variable
- **WHEN** user executes `make harvester` without providing SITE variable
- **THEN** system displays usage message indicating SITE variable is required and exits with error

### Requirement: Infrastructure initialization is shared across commands
Both pioneer and harvester commands SHALL initialize database and storage connections using environment variables with sensible defaults.

#### Scenario: Commands use environment variables for configuration
- **WHEN** either pioneer or harvester command is executed
- **THEN** system reads DATABASE_URL, S3_ENDPOINT, S3_REGION, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY, and S3_PUBLIC_URL from environment variables or uses documented default values

#### Scenario: Database connection fails
- **WHEN** either pioneer or harvester command is executed and database connection fails
- **THEN** system logs a clear error message and exits with non-zero code before attempting to run the crawler

#### Scenario: Storage connection fails
- **WHEN** either pioneer or harvester command is executed and storage connection fails
- **THEN** system logs a clear error message and exits with non-zero code before attempting to run the crawler
