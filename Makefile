# Fugue — 로컬 개발 명령어
# make dev  → 모든 서비스 띄우고 브라우저 열기

API_DIR = apps/api
WEB_DIR = apps/web
DB_URL = postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable

.PHONY: dev dev-infra dev-api dev-web dev-stop seed migrate test

# ============================================================
# 한 방에 전부 띄우기
# ============================================================
dev: dev-infra migrate seed dev-api dev-web
	@echo ""
	@echo "🐡 Fugue is running!"
	@echo "   Frontend: http://localhost:3000"
	@echo "   API:      http://localhost:8080"
	@echo ""
	@echo "   make dev-stop  → 전부 종료"
	@open http://localhost:3000

# ============================================================
# 인프라 (PostgreSQL + Redis)
# ============================================================
dev-infra:
	@echo "🐘 Starting PostgreSQL + Redis..."
	@docker-compose up -d
	@echo "⏳ Waiting for PostgreSQL..."
	@until docker-compose exec -T postgres pg_isready -U fugue > /dev/null 2>&1; do sleep 0.5; done
	@echo "✅ PostgreSQL ready"

# ============================================================
# DB 마이그레이션 + 시드
# ============================================================
migrate:
	@echo "📦 Running migrations..."
	@cd $(API_DIR) && migrate -path db/migrations -database "$(DB_URL)" up 2>&1 | grep -v "no change" || true

seed:
	@echo "🌱 Seeding data..."
	@docker-compose exec -T postgres psql -U fugue -d fugue -v ON_ERROR_STOP=1 < $(API_DIR)/db/seed.sql > /dev/null

# ============================================================
# Go API 서버 (백그라운드)
# ============================================================
dev-api:
	@echo "🚀 Starting Go API on :8080..."
	@cd $(API_DIR) && export $$(grep -v '^\#' .env.dev | xargs) && go run cmd/server/main.go &
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
	@cd $(API_DIR) && go test ./internal/works/... -v
	@echo ""
	@echo "🧪 Running Frontend tests..."
	@cd $(WEB_DIR) && npm test
