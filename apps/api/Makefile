DB_URL ?= postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable
MIGRATIONS_DIR = db/migrations

.PHONY: migrate-up migrate-down migrate-create lint fmt setup

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

setup:
	cd ../.. && lefthook install
