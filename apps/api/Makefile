DB_URL ?= postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable
DB_CONTAINER ?= fugue-postgres-1
DB_USER ?= fugue
DB_NAME ?= fugue
MIGRATIONS_DIR = db/migrations

.PHONY: migrate-up migrate-down migrate-create lint fmt setup seed

migrate-up:
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

migrate-down:
	yes | migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $$name

lint:
	golangci-lint run ./...

fmt:
	goimports -w .

seed:
	docker exec -i $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME) -v ON_ERROR_STOP=1 < db/seed.sql

setup:
	cd ../.. && lefthook install
