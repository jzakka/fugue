package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

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
	// spec: docs/architecture.md Rate Limit "핀 생성: 30/분/유저" — per-creator, not per-IP.
	// spec: ratelimit `유저 단위 빈도 제한 surface를 노출한다`
	r.With(auth.JWTMiddleware(jwtSvc), pinRL.MiddlewareByCreatorID).Post("/api/pins", pinHandler.Create)
	// spec: interaction `미인증 호출자의 핀 조회에는 interaction이 기록되지 않는다`
	// OptionalJWTMiddleware exposes auth context when present so GetByID can piggyback view interaction.
	r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/api/pins/{id}", pinHandler.GetByID)
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
		// spec: board `공개 보드 조회 라우트는 선택적 인증 미들웨어로 보호된다`
		r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/", boardsHandler.ListByCreator)
		r.With(auth.JWTMiddleware(jwtSvc)).Post("/", boardsHandler.Create)
		// spec: board `공개 보드 조회 라우트는 선택적 인증 미들웨어로 보호된다`
		r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/{id}", boardsHandler.GetByID)
		r.With(auth.JWTMiddleware(jwtSvc)).Put("/{id}", boardsHandler.Update)
		r.With(auth.JWTMiddleware(jwtSvc)).Delete("/{id}", boardsHandler.Delete)
		r.With(auth.JWTMiddleware(jwtSvc)).Post("/{id}/pins", boardsHandler.AddPin)
		r.With(auth.JWTMiddleware(jwtSvc)).Delete("/{id}/pins/{pin_id}", boardsHandler.RemovePin)
	})

	// Interactions
	r.With(auth.JWTMiddleware(jwtSvc)).Post("/api/interactions", interactionHandler.Create)

	// Feed (personalized)
	// spec: feed `피드 라우트는 선택적 인증 미들웨어로 보호된다`
	r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/api/feed", feedHandler.GetFeed)

	// Auth routes
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/providers", authHandler.Providers)
		r.With(authRL.Middleware).Get("/{provider}/login", authHandler.Login)
		r.With(callbackRL.Middleware).Get("/{provider}/callback", authHandler.Callback)
		r.Post("/refresh", authHandler.Refresh)
		r.With(authRL.Middleware).Post("/logout", authHandler.Logout)
		r.With(auth.JWTMiddleware(jwtSvc)).Get("/me", authHandler.Me)
	})

	// Graceful shutdown wiring. cmd/bot/main.go (L276) already uses the
	// signal.NotifyContext + http.Server.Shutdown pattern; cmd/server now
	// matches it so SIGTERM during a k8s rolling deploy drains in-flight
	// HTTP handlers instead of cutting them mid-write, and so the deferred
	// db.Close()/rdb.Close() above actually run on shutdown (bare
	// http.ListenAndServe + log.Fatalf would call os.Exit and skip them).
	// 25s drain leaves ~5s headroom inside the k8s default
	// terminationGracePeriodSeconds=30s for the runtime to finish exiting.
	//
	// BaseContext wires the signal ctx into every accepted connection so
	// SIGTERM propagates into each in-flight r.Context(): srv.Shutdown alone
	// only stops the listener and waits for handlers — it does NOT cancel
	// req.Context. Without BaseContext, in-flight handlers (cycle 120 PR
	// #353 GoogleProvider.FetchProfile, cycle 127 PR #377 pin.Create
	// ffmpeg/ffprobe) see ctx cancel only on client-disconnect, not on
	// server shutdown. With BaseContext, the cancel chain is
	// signal ctx → conn ctx (via BaseContext) → req.Context (via the
	// per-conn serve loop), so CommandContext kills ffmpeg subprocesses
	// and Shutdown drains within actual response latency instead of
	// waiting up to 25s for ffmpeg's natural exit. cmd/bot/main.go's
	// signal.NotifyContext → DequeueCtx → processOne chain is the sister
	// precedent on the bot track.
	addr := ":" + cfg.Port

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("fugue api server listening on %s", addr)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	case <-ctx.Done():
		log.Printf("fugue api server: shutdown signal received, draining...")
		shutdownCtx, cancelShutdown := context.WithTimeout(
			context.Background(), 25*time.Second)
		defer cancelShutdown()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("fugue api server: graceful shutdown failed: %v", err)
		}
		log.Printf("fugue api server: shutdown complete")
	}
}
