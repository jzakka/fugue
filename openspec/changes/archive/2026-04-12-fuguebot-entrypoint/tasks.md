## 1. Dependencies and Setup

- [x] 1.1 Add github.com/spf13/cobra dependency to go.mod
- [x] 1.2 Run go mod tidy to update dependencies

## 2. Cobra CLI Structure and Site Registry

- [x] 2.1 Refactor cmd/bot/main.go to use Cobra root command with help text
- [x] 2.2 Create source name to domain registry map (unsplash → unsplash.com, fma → freemusicarchive.org)
- [x] 2.3 Implement resolveDomain() helper function that handles both source names and direct domains
- [x] 2.4 Extract common infrastructure initialization (DB, Storage) into reusable function
- [x] 2.5 Implement pioneer subcommand with site argument validation
- [x] 2.6 Implement harvester subcommand with site argument validation

## 3. Pioneer Command Implementation

- [x] 3.1 Initialize Pioneer dependencies (SiteRepository, GraphRepository, ScriptRepository, AIClient, ScriptExecutor)
- [x] 3.2 Resolve site name to domain using resolveDomain()
- [x] 3.3 Verify site exists in database by domain before running pioneer
- [x] 3.4 Create Pioneer instance with appropriate config
- [x] 3.5 Execute pioneer.Run() with site_id, context and error handling

## 4. Harvester Command Implementation

- [x] 4.1 Initialize Harvester dependencies (SiteRepository, GraphRepository, ScriptRepository, ScriptExecutor, Pipeline)
- [x] 4.2 Resolve site name to domain using resolveDomain()
- [x] 4.3 Verify site exists in database by domain before running harvester
- [x] 4.4 Create Harvester instance with appropriate config
- [x] 4.5 Execute harvester.Run() with site_id, context and error handling

## 5. Makefile Integration

- [x] 5.1 Add pioneer target to apps/api/Makefile with SITE variable validation
- [x] 5.2 Add harvester target to apps/api/Makefile with SITE variable validation
- [x] 5.3 Test make pioneer SITE=<test-site> command
- [x] 5.4 Test make harvester SITE=<test-site> command

## 6. Error Handling and Validation

- [x] 6.1 Implement clear error messages for missing site argument
- [x] 6.2 Implement clear error messages for non-existent site
- [x] 6.3 Implement graceful error handling for infrastructure connection failures
- [x] 6.4 Add logging for command execution start and completion

## 7. Testing and Documentation

- [x] 7.1 Test pioneer command with valid site
- [x] 7.2 Test harvester command with valid site
- [x] 7.3 Test error cases (missing site, invalid site, connection failures)
- [x] 7.4 Update README or add usage documentation for new CLI commands
