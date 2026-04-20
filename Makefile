# Fugue — 로컬 개발 명령어
# make dev  → 모든 서비스 띄우고 브라우저 열기

API_DIR = apps/api
WEB_DIR = apps/web
DB_URL = postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable

.PHONY: dev dev-kill dev-infra dev-api dev-web dev-stop seed migrate test show-map pioneer harvester \
	migrate-up migrate-down migrate-create lint fmt setup

# ============================================================
# 한 방에 전부 띄우기
# ============================================================
dev: dev-kill dev-infra migrate seed dev-api dev-web
	@echo ""
	@echo "🐡 Fugue is running!"
	@echo "   Frontend: http://localhost:3000"
	@echo "   API:      http://localhost:8080"
	@echo ""
	@echo "   make dev-stop  → 전부 종료"
	@open http://localhost:3000

# ============================================================
# 이전 프로세스 정리 (포트 충돌 방지)
# ============================================================
dev-kill:
	@echo "🧹 Killing stale processes on :8080 / :3000..."
	@-lsof -ti :8080 | xargs kill 2>/dev/null || true
	@-lsof -ti :3000 | xargs kill 2>/dev/null || true
	@-pkill -f "go run cmd/server/main.go" 2>/dev/null || true
	@-pkill -f "next dev" 2>/dev/null || true
	@echo "✅ Ports cleared"

# ============================================================
# 인프라 (PostgreSQL + Redis)
# ============================================================
dev-infra:
	@echo "🐘 Starting PostgreSQL + Redis..."
	@docker-compose up -d
	@echo "⏳ Waiting for PostgreSQL (container)..."
	@until docker-compose exec -T postgres pg_isready -U fugue > /dev/null 2>&1; do sleep 0.5; done
	@echo "⏳ Waiting for PostgreSQL (host port)..."
	@until nc -z localhost 5432 2>/dev/null; do sleep 0.5; done
	@sleep 1
	@echo "✅ PostgreSQL ready"

# ============================================================
# DB 마이그레이션 + 시드
# ============================================================
migrate:
	@echo "📦 Running migrations..."
	@cd $(API_DIR) && migrate -path db/migrations -database "$(DB_URL)" up 2>&1; \
	EXIT=$$?; \
	if [ $$EXIT -ne 0 ]; then \
		echo "❌ Migration failed (exit $$EXIT). Check database connection and migration files."; exit 1; \
	fi

seed:
	@echo "🌱 Seeding tags..."
	@docker-compose exec -T postgres psql -U fugue -d fugue -v ON_ERROR_STOP=1 < $(API_DIR)/db/seed_tags.sql > /dev/null
	@echo "🌱 Seeding data..."
	@docker-compose exec -T postgres psql -U fugue -d fugue -v ON_ERROR_STOP=1 < $(API_DIR)/db/seed.sql > /dev/null

# ============================================================
# Go API 서버 (백그라운드)
# ============================================================
dev-api:
	@echo "🚀 Starting Go API on :8080..."
	@cd $(API_DIR) && export $$(grep -v '^\#' $$([ -f .env ] && echo .env || echo .env.dev) | xargs) && go run cmd/server/main.go &
	@sleep 2
	@curl -sf http://localhost:8080/health > /dev/null && echo "✅ API ready" || echo "⏳ API starting..."

# ============================================================
# Next.js 프론트엔드 (백그라운드)
# ============================================================
dev-web:
	@echo "🌐 Starting Next.js on :3000..."
	@cd $(WEB_DIR) && npm run dev &
	@sleep 2

# ============================================================
# 전부 종료
# ============================================================
dev-stop:
	@echo "🛑 Stopping all services..."
	@-pkill -f "go run cmd/server/main.go" 2>/dev/null || true
	@-pkill -f "next dev" 2>/dev/null || true
	@docker-compose down
	@echo "✅ All stopped"

# ============================================================
# 테스트
# ============================================================
test:
	@echo "🧪 Running Go tests..."
	@cd $(API_DIR) && go test ./internal/... -v
	@echo ""
	@echo "🧪 Running Frontend tests..."
	@cd $(WEB_DIR) && npm test

# ============================================================
# Bot Graph Visualization
# ============================================================
show-map:
	@echo "🗺️  Generating bot graph visualization..."
	@cd $(API_DIR) && export $$(grep -v '^\#' $$([ -f .env ] && echo .env || echo .env.dev) | xargs) && \
		go run cmd/bot-visualize/main.go -output=../../graph.html
	@echo ""
	@echo "   Tip: Use -format=png or -format=svg for image export (requires Graphviz)"
	@echo "   Tip: Use -filter-site=<domain> to show only one site"

# ============================================================
# Bot Crawlers (Pioneer/Harvester)
# ============================================================
pioneer:
	@if [ -z "$(SITE)" ]; then echo "Usage: make pioneer SITE=<site>"; exit 1; fi
	@echo "🔍 Running Pioneer for $(SITE)..."
	@cd $(API_DIR) && export $$(grep -v '^\#' $$([ -f .env ] && echo .env || echo .env.dev) | xargs) && \
		go run cmd/bot/main.go pioneer $(SITE)

harvester:
	@if [ -z "$(SITE)" ]; then echo "Usage: make harvester SITE=<site>"; exit 1; fi
	@echo "🌾 Running Harvester for $(SITE)..."
	@cd $(API_DIR) && export $$(grep -v '^\#' $$([ -f .env ] && echo .env || echo .env.dev) | xargs) && \
		HARVESTER_MODE=real go run cmd/bot/main.go harvester $(SITE)

# ============================================================
# DB 마이그레이션 (개별 제어)
# ============================================================
migrate-up:
	@cd $(API_DIR) && migrate -path db/migrations -database "$(DB_URL)" up

migrate-down:
	@cd $(API_DIR) && yes | migrate -path db/migrations -database "$(DB_URL)" down

migrate-create:
	@read -p "Migration name: " name; \
	cd $(API_DIR) && migrate create -ext sql -dir db/migrations -seq $$name

# ============================================================
# Go 코드 품질
# ============================================================
lint:
	@cd $(API_DIR) && golangci-lint run ./...

fmt:
	@cd $(API_DIR) && goimports -w .

# ============================================================
# 프로젝트 설정
# ============================================================
setup:
	@lefthook install
	@echo "✅ Git hooks installed"

