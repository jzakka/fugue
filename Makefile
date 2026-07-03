# Fugue — 로컬 개발 명령어
# make dev  → 모든 서비스 띄우고 브라우저 열기

API_DIR = apps/api
WEB_DIR = apps/web
DB_URL = postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable

.PHONY: dev dev-kill dev-infra dev-api dev-web dev-stop migrate seed test show-map pioneer harvester \
	migrate-up migrate-down migrate-create lint fmt setup fuguebot-progress fuguebot-graph \
	ensure-infra crawl-status crawl

# ============================================================
# 한 방에 전부 띄우기
# ============================================================
dev: dev-kill dev-infra migrate dev-api dev-web
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

# 수동 전용 — dev/ensure-infra 에서 자동 호출하지 않음 (기존 데이터 TRUNCATE 주의)
seed:
	@echo "⚠️  Seeding wipes pins/creators/tags data (TRUNCATE)"
	@echo "🌱 Seeding tags..."
	@docker-compose exec -T postgres psql -U fugue -d fugue -v ON_ERROR_STOP=1 < $(API_DIR)/db/seed_tags.sql > /dev/null
	@echo "🌱 Seeding data..."
	@docker-compose exec -T postgres psql -U fugue -d fugue -v ON_ERROR_STOP=1 < $(API_DIR)/db/seed.sql > /dev/null
	@echo "✅ Seed complete"

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
# 크롤러 부트스트랩 (Pioneer/Harvester 가 자동으로 호출)
# - localhost:5432 에 Postgres 가 떠있는지 검사 (워크트리 외부 컨테이너 포함)
# - 없으면 워크트리 compose 로 dev-infra + migrate
# - 있으면 migrate 만 빠르게 멱등 적용 (이미 적용된 것은 no-op)
# - 마지막으로 bot_sites 시드 적용 (멱등, ON CONFLICT DO NOTHING)
# ============================================================
ensure-infra:
	@PG_CONTAINER=$$(docker ps --filter "publish=5432" --format '{{.Names}}' | head -n1); \
	if [ -z "$$PG_CONTAINER" ]; then \
		echo "🐘 Postgres not running — bootstrapping infrastructure..."; \
		$(MAKE) --no-print-directory dev-infra; \
		$(MAKE) --no-print-directory migrate; \
		PG_CONTAINER=$$(docker ps --filter "publish=5432" --format '{{.Names}}' | head -n1); \
	else \
		echo "🐘 Reusing existing Postgres container: $$PG_CONTAINER"; \
		(cd $(API_DIR) && migrate -path db/migrations -database "$(DB_URL)" up >/dev/null 2>&1 || true); \
	fi; \
	docker exec -i $$PG_CONTAINER psql -U fugue -d fugue -v ON_ERROR_STOP=1 \
		< $(API_DIR)/db/seed_bot_sites.sql > /dev/null
	@echo "✅ Infra ready (bot_sites seeded)"

# ============================================================
# Bot Crawlers (Pioneer/Harvester)
# - 인프라가 안 떠있으면 ensure-infra 가 알아서 띄움
# - Pioneer 는 SITE 별칭 필수 (unsplash | fma | pixiv)
# - Harvester 는 모든 사이트의 URL을 한 워커가 우선순위 순으로 소비
# ============================================================
pioneer: ensure-infra
	@if [ -z "$(SITE)" ]; then echo "Usage: make pioneer SITE=<unsplash|fma|pixiv>"; exit 1; fi
	@echo "🔍 Running Pioneer for $(SITE)..."
	@cd $(API_DIR) && export $$(grep -v '^\#' $$([ -f .env ] && echo .env || echo .env.dev) | xargs) && \
		GOWORK=off go run ./cmd/bot pioneer $(SITE)

harvester: ensure-infra
	@echo "🌾 Running Harvester worker (all sites)..."
	@cd $(API_DIR) && export $$(grep -v '^\#' $$([ -f .env ] && echo .env || echo .env.dev) | xargs) && \
		HARVESTER_MODE=real GOWORK=off go run ./cmd/bot harvester

# ============================================================
# 지속 크롤링 — DURATION 동안 Pioneer/Harvester 를 무한 루프로 돌림
# - 잡 본체는 100건 처리 후 자체 종료(worker budget) → 루프가 즉시 새 잡 시작
# - DURATION 경과 시 SIGTERM 으로 두 루프 + 자식 워커 모두 종료
# - DURATION 포맷: 30s / 5m / 1h (BSD/GNU date 차이 회피 위해 직접 초 계산)
# - 사용: make crawl SITE=<unsplash|fma|pixiv> [DURATION=10m]
# ============================================================
DURATION ?= 5m

crawl: ensure-infra
	@if [ -z "$(SITE)" ]; then echo "Usage: make crawl SITE=<unsplash|fma|pixiv> [DURATION=10m]"; exit 1; fi
	@echo "🐡 Crawling SITE=$(SITE) for $(DURATION)..."
	@DUR=$(DURATION); \
	 case "$$DUR" in \
	   *s) secs=$${DUR%s} ;; \
	   *m) secs=$$(( $${DUR%m} * 60 )) ;; \
	   *h) secs=$$(( $${DUR%h} * 3600 )) ;; \
	   *)  secs=$$DUR ;; \
	 esac; \
	 end=$$(( $$(date +%s) + secs )); \
	 stop_workers='pkill -TERM -f "bot pioneer" 2>/dev/null; pkill -TERM -f "bot harvester" 2>/dev/null; sleep 2; pkill -KILL -f "bot pioneer" 2>/dev/null; pkill -KILL -f "bot harvester" 2>/dev/null'; \
	 trap "$$stop_workers" INT TERM EXIT; \
	 ( while [ $$(date +%s) -lt $$end ]; do $(MAKE) --no-print-directory pioneer   SITE=$(SITE) || true; done ) & \
	 ( while [ $$(date +%s) -lt $$end ]; do $(MAKE) --no-print-directory harvester             || true; done ) & \
	 while [ $$(date +%s) -lt $$end ]; do sleep 5; done; \
	 eval "$$stop_workers"; \
	 wait 2>/dev/null || true
	@echo "🛑 Stopped after $(DURATION)."
	@$(MAKE) --no-print-directory crawl-status

# ============================================================
# 현재 크롤링 상태 (한 번 보고 종료)
# - 큐별 pending / done / dead 카운트
# - 누적 봇 Pin 수 + 최근 생성된 Pin 5개
# ============================================================
crawl-status:
	@echo ""
	@echo "🐡 Crawl Status"
	@echo "═══════════════════════════════════════════════════════════════"
	@PG_CONTAINER=$$(docker ps --filter "publish=5432" --format '{{.Names}}' | head -n1); \
	if [ -z "$$PG_CONTAINER" ]; then echo "(Postgres not running — run \`make pioneer\` or \`make harvester\` to bootstrap)"; exit 0; fi; \
	docker exec -i $$PG_CONTAINER psql -U fugue -d fugue -c "\
SELECT 'pioneer'   AS queue, \
       COUNT(*) FILTER (WHERE last_fetched_at IS NULL AND fetch_error_count < 5) AS pending, \
       COUNT(*) FILTER (WHERE last_fetched_at IS NOT NULL)                       AS done, \
       COUNT(*) FILTER (WHERE fetch_error_count >= 5)                            AS dead \
  FROM pioneer_frontier \
UNION ALL \
SELECT 'harvester', \
       COUNT(*) FILTER (WHERE harvested_at IS NULL AND harvest_error_count < 5), \
       COUNT(*) FILTER (WHERE harvested_at IS NOT NULL), \
       COUNT(*) FILTER (WHERE harvest_error_count >= 5) \
  FROM harvester_frontier;"; \
	docker exec -i $$PG_CONTAINER psql -U fugue -d fugue -c "\
SELECT COUNT(*) AS bot_pins, MAX(created_at) AS last_pin_at \
  FROM pins \
 WHERE creator_id = (SELECT id FROM creators WHERE nickname='fuguebot' LIMIT 1);"; \
	docker exec -i $$PG_CONTAINER psql -U fugue -d fugue -c "\
SELECT LEFT(id::text, 8) AS id, LEFT(title, 40) AS title, media_type, created_at \
  FROM pins \
 WHERE creator_id = (SELECT id FROM creators WHERE nickname='fuguebot' LIMIT 1) \
 ORDER BY created_at DESC LIMIT 5;"

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

# ============================================================
# Fuguebot Scheduler 진행 현황
# ============================================================
fuguebot-progress:
	@printf '\n'
	@printf '══════════════════════════════════════════════════════════════════\n'
	@printf '  Fuguebot Pioneer / Harvester Scheduler 진행 현황\n'
	@printf '══════════════════════════════════════════════════════════════════\n'
	@printf '\n'
	@printf '[ 완료 (archive) ]\n'
	@printf '──────────────────────────────────────────────────────────────────\n'
	@for dir in openspec/changes/archive/*/; do \
		name=$$(basename "$$dir"); \
		case "$$name" in \
			2026-04-17-*|2026-04-18-*|2026-04-19-*|2026-04-20-*) \
				printf '  [x] %s\n' "$$name" ;; \
		esac; \
	done
	@printf '\n'
	@printf '[ 진행 중 (active) ]\n'
	@printf '──────────────────────────────────────────────────────────────────\n'
	@total_done=0; total_all=0; \
	for dir in openspec/changes/*/; do \
		name=$$(basename "$$dir"); \
		[ "$$name" = "archive" ] && continue; \
		tasks_file="$$dir/tasks.md"; \
		if [ ! -f "$$tasks_file" ]; then \
			printf '  (!) %-40s tasks.md 없음\n' "$$name"; \
			continue; \
		fi; \
		done_n=$$(grep -e '- \[x\]' -c "$$tasks_file" 2>/dev/null); done_n=$${done_n:-0}; \
		inp_n=$$(grep -e '- \[~\]' -c "$$tasks_file" 2>/dev/null); inp_n=$${inp_n:-0}; \
		todo_n=$$(grep -e '- \[ \]' -c "$$tasks_file" 2>/dev/null); todo_n=$${todo_n:-0}; \
		all_n=$$((done_n + inp_n + todo_n)); \
		[ "$$all_n" -eq 0 ] && continue; \
		pct=$$((done_n * 100 / all_n)); \
		filled=$$((pct * 20 / 100)); empty=$$((20 - filled)); \
		bar=""; i=0; \
		while [ "$$i" -lt "$$filled" ]; do bar="$${bar}#"; i=$$((i+1)); done; \
		while [ "$$i" -lt 20 ]; do bar="$${bar}-"; i=$$((i+1)); done; \
		if [ "$$inp_n" -gt 0 ]; then st="~"; \
		elif [ "$$done_n" -eq "$$all_n" ]; then st="x"; \
		else st=" "; fi; \
		printf '  [%s] %-40s [%s] %3d%% (%d/%d)\n' "$$st" "$$name" "$$bar" "$$pct" "$$done_n" "$$all_n"; \
		[ "$$inp_n" -gt 0 ] && printf '            진행중 %d건\n' "$$inp_n"; \
		total_done=$$((total_done + done_n)); \
		total_all=$$((total_all + all_n)); \
	done; \
	printf '\n'; \
	printf '──────────────────────────────────────────────────────────────────\n'; \
	if [ "$$total_all" -gt 0 ]; then \
		total_pct=$$((total_done * 100 / total_all)); \
		printf '  전체: %d / %d tasks  (%d%%)\n' "$$total_done" "$$total_all" "$$total_pct"; \
	fi
	@$(MAKE) --no-print-directory fuguebot-graph

# ============================================================
# Fuguebot 의존성 그래프 (HTML 비주얼)
# ============================================================
fuguebot-graph:
	@python3 fuguebot_graph.py
