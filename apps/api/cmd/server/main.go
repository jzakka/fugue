package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	"github.com/chungsanghwa/fugue/apps/api/internal/config"
)

func main() {
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
		"discord": auth.NewDiscordProvider(
			cfg.DiscordClientID,
			cfg.DiscordClientSecret,
			cfg.OAuthCallbackBase+"/api/auth/discord/callback",
		),
	}

	authHandler := auth.NewHandler(providers, stateManager, authService, jwtSvc, cfg.FrontendURL, cfg.IsDevMode())

	// Rate limiters
	authRL := auth.NewRateLimiter(rdb, 10, time.Minute)
	callbackRL := auth.NewRateLimiter(rdb, 5, time.Minute)
	refreshRL := auth.NewRateLimiter(rdb, 20, time.Minute)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	// Auth routes (no JWT middleware)
	r.Route("/api/auth", func(r chi.Router) {
		r.With(authRL.Middleware).Get("/{provider}/login", authHandler.Login)
		r.With(callbackRL.Middleware).Get("/{provider}/callback", authHandler.Callback)
		r.With(refreshRL.Middleware).Post("/refresh", authHandler.Refresh)
		r.With(authRL.Middleware).Post("/logout", authHandler.Logout)
	})

	// Protected API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(auth.JWTMiddleware(jwtSvc))
		// Future: creators, works, recommend, og endpoints
	})

	addr := ":" + cfg.Port
	log.Printf("fugue api server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
