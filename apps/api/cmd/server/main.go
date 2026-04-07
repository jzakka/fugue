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

	"github.com/chungsanghwa/fugue/apps/api/internal/admin"
	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	"github.com/chungsanghwa/fugue/apps/api/internal/boards"
	"github.com/chungsanghwa/fugue/apps/api/internal/config"
	"github.com/chungsanghwa/fugue/apps/api/internal/creator"
	"github.com/chungsanghwa/fugue/apps/api/internal/feed"
	"github.com/chungsanghwa/fugue/apps/api/internal/interaction"
	"github.com/chungsanghwa/fugue/apps/api/internal/og"
	"github.com/chungsanghwa/fugue/apps/api/internal/pin"
	"github.com/chungsanghwa/fugue/apps/api/internal/search"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
	"github.com/chungsanghwa/fugue/apps/api/internal/tag"
)

func main() {
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

	// Storage (S3/MinIO)
	store, err := storage.NewClient(storage.Config{
		Endpoint:  cfg.S3Endpoint,
		Region:    cfg.S3Region,
		Bucket:    cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		PublicURL: cfg.S3PublicURL,
	})
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

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
	ogRL := auth.NewRateLimiter(rdb, 20, time.Minute)
	pinRL := auth.NewRateLimiter(rdb, 30, time.Minute)

	// Handlers
	pinHandler := pin.NewHandler(db, store)
	creatorHandler := creator.NewHandler(db)
	ogHandler := og.NewHandler()
	boardsHandler := boards.NewHandler(db)
	interactionHandler := interaction.NewHandler(db)
	tagHandler := tag.NewHandler(db)
	searchHandler := search.NewHandler(db)
	feedHandler := feed.NewHandler(db, rdb)
	adminHandler := admin.NewHandler(db)

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

	// Search
	r.Get("/api/search", searchHandler.Search)

	// Tag routes
	r.Get("/api/tags/popular", tagHandler.PopularTags)
	r.Get("/api/tags", tagHandler.List)

	// Pin routes
	r.Get("/api/pins", pinHandler.List)
	r.With(auth.JWTMiddleware(jwtSvc), pinRL.Middleware).Post("/api/pins", pinHandler.Create)
	r.Get("/api/pins/{id}", pinHandler.GetByID)
	r.With(auth.JWTMiddleware(jwtSvc)).Delete("/api/pins/{id}", pinHandler.Delete)
	r.Get("/api/pins/{id}/related", pinHandler.Related)
	r.Get("/api/pins/{id}/boards", boardsHandler.ListByPin)

	// OG fetch
	r.With(ogRL.Middleware).Post("/api/og/fetch", ogHandler.Fetch)

	// Creator routes
	r.Route("/api/creators", func(r chi.Router) {
		r.With(auth.JWTMiddleware(jwtSvc)).Get("/me", creatorHandler.GetMe)
		r.With(auth.JWTMiddleware(jwtSvc)).Put("/me", creatorHandler.UpdateMe)
		r.Get("/{id}", creatorHandler.GetByID)
	})

	// Board routes
	r.Route("/api/boards", func(r chi.Router) {
		r.Get("/", boardsHandler.ListByCreator)
		r.With(auth.JWTMiddleware(jwtSvc)).Post("/", boardsHandler.Create)
		r.Get("/{id}", boardsHandler.GetByID)
		r.With(auth.JWTMiddleware(jwtSvc)).Put("/{id}", boardsHandler.Update)
		r.With(auth.JWTMiddleware(jwtSvc)).Delete("/{id}", boardsHandler.Delete)
		r.With(auth.JWTMiddleware(jwtSvc)).Post("/{id}/pins", boardsHandler.AddPin)
		r.With(auth.JWTMiddleware(jwtSvc)).Delete("/{id}/pins/{pin_id}", boardsHandler.RemovePin)
	})

	// Interactions
	r.With(auth.JWTMiddleware(jwtSvc)).Post("/api/interactions", interactionHandler.Create)

	// Feed (personalized)
	r.Get("/api/feed", feedHandler.GetFeed)

	// Admin routes (X-Admin-Key auth)
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(admin.AdminKeyMiddleware)
		r.Get("/bot/status", adminHandler.Status)
		r.Get("/bot/sources", adminHandler.ListSources)
		r.Post("/bot/sources", adminHandler.CreateSource)
		r.Patch("/bot/sources/{id}", adminHandler.ToggleSource)
		r.Delete("/bot/sources/{id}", adminHandler.DeleteSource)
	})

	// Auth routes
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/providers", authHandler.Providers)
		r.With(authRL.Middleware).Get("/{provider}/login", authHandler.Login)
		r.With(callbackRL.Middleware).Get("/{provider}/callback", authHandler.Callback)
		r.Post("/refresh", authHandler.Refresh)
		r.With(authRL.Middleware).Post("/logout", authHandler.Logout)
		r.With(auth.JWTMiddleware(jwtSvc)).Get("/me", authHandler.Me)
	})

	addr := ":" + cfg.Port
	log.Printf("fugue api server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
