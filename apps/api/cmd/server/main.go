package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	"github.com/chungsanghwa/fugue/apps/api/internal/config"
	"github.com/chungsanghwa/fugue/apps/api/internal/creator"
	"github.com/chungsanghwa/fugue/apps/api/internal/works"
)

func main() {
	// Load .env file if present (ignored in production where env vars are set directly)
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("database ping: %v", err)
	}

	// Redis
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	opt.PoolSize = 10
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()

	// Auth setup
	jwtSvc := auth.NewJWTService(cfg.JWTSecret)
	stateManager := auth.NewStateManager(rdb)
	authService := auth.NewService(db, rdb, jwtSvc)

	providers := map[string]auth.Provider{
		"google": auth.NewGoogleProvider(
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			cfg.OAuthCallbackBase+"/api/auth/google/callback",
		),
	}
	// TODO: Discord를 다시 필수로 변경할 것 (OAuth 앱 등록 후)
	if cfg.DiscordClientID != "" && cfg.DiscordClientSecret != "" {
		providers["discord"] = auth.NewDiscordProvider(
			cfg.DiscordClientID,
			cfg.DiscordClientSecret,
			cfg.OAuthCallbackBase+"/api/auth/discord/callback",
		)
	}

	authHandler := auth.NewHandler(providers, stateManager, authService, jwtSvc, cfg.FrontendURL, cfg.IsDevMode())

	// Rate limiters
	authRL := auth.NewRateLimiter(rdb, 10, time.Minute)
	callbackRL := auth.NewRateLimiter(rdb, 5, time.Minute)

	// Works handler
	worksHandler := works.NewHandler(db)

	// Creator handler
	creatorHandler := creator.NewHandler(db)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	// Public API routes (no JWT, no rate limit — SSR requests come from
	// the Next.js server IP and would exhaust a shared bucket under normal traffic)
	r.Get("/api/works", worksHandler.List)

	// Creator routes — /me (protected) must be registered before /{id}
	// so Chi's trie matches the literal "me" before the param.
	r.Route("/api/creators", func(r chi.Router) {
		r.With(auth.JWTMiddleware(jwtSvc)).Get("/me", creatorHandler.GetMe)
		r.With(auth.JWTMiddleware(jwtSvc)).Put("/me", creatorHandler.UpdateMe)
		r.Get("/{id}", creatorHandler.GetByID)
	})

	// Auth routes — /me must live here because Chi's Route("/api/auth")
	// subrouter captures all /api/auth/* requests, making any /api/auth/me
	// registered under Route("/api") unreachable (404).
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/providers", authHandler.Providers)
		r.With(authRL.Middleware).Get("/{provider}/login", authHandler.Login)
		r.With(callbackRL.Middleware).Get("/{provider}/callback", authHandler.Callback)
		// No rate limit on refresh — the refresh token itself is the auth.
		// SSR calls this from the Next.js server IP, which would exhaust a
		// shared bucket under normal multi-user traffic.
		r.Post("/refresh", authHandler.Refresh)
		r.With(authRL.Middleware).Post("/logout", authHandler.Logout)

		// Protected: requires valid JWT
		r.With(auth.JWTMiddleware(jwtSvc)).Get("/me", authHandler.Me)
	})

	addr := ":" + cfg.Port
	log.Printf("fugue api server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
