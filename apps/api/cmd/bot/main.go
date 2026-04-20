package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

// Source name to domain registry
var sourceRegistry = map[string]string{
	"unsplash": "unsplash.com",
	"fma":      "freemusicarchive.org",
	"pixiv":    "pixiv.net",
}

// resolveDomain resolves a source name to a domain
func resolveDomain(name string) (string, error) {
	if domain, ok := sourceRegistry[name]; ok {
		return domain, nil
	}
	// name이 이미 domain 형식이면 그대로 사용
	if strings.Contains(name, ".") {
		return name, nil
	}

	// List available sources
	available := make([]string, 0, len(sourceRegistry))
	for k := range sourceRegistry {
		available = append(available, k)
	}
	return "", fmt.Errorf("unknown site: %s (available: %v)", name, available)
}

// Infrastructure holds common infrastructure dependencies
type Infrastructure struct {
	DB      *sql.DB
	Storage *storage.Client
	Queries *db.Queries
}

// initInfrastructure initializes database and storage connections
func initInfrastructure() (*Infrastructure, error) {
	_ = godotenv.Load()

	// Database
	dbURL := envOrDefault("DATABASE_URL", "postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable")
	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("database open: %w", err)
	}
	database.SetMaxOpenConns(5)
	database.SetMaxIdleConns(2)
	database.SetConnMaxLifetime(5 * time.Minute)

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}

	// Storage (S3/MinIO)
	store, err := storage.NewClient(storage.Config{
		Endpoint:  envOrDefault("S3_ENDPOINT", "http://localhost:9000"),
		Region:    envOrDefault("S3_REGION", "us-east-1"),
		Bucket:    envOrDefault("S3_BUCKET", "fugue-media"),
		AccessKey: envOrDefault("S3_ACCESS_KEY", "fugue"),
		SecretKey: envOrDefault("S3_SECRET_KEY", "fuguedev123"),
		PublicURL: envOrDefault("S3_PUBLIC_URL", "http://localhost:9000/fugue-media"),
	})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("storage: %w", err)
	}

	queries := db.New(database)

	return &Infrastructure{
		DB:      database,
		Storage: store,
		Queries: queries,
	}, nil
}

func (infra *Infrastructure) Close() {
	if infra.DB != nil {
		_ = infra.DB.Close()
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var rootCmd = &cobra.Command{
	Use:   "fuguebot",
	Short: "Fugue bot crawler CLI",
	Long: `Fuguebot manages Pioneer (exploration) and Harvester (extraction) crawlers.
	
Examples:
  fuguebot pioneer unsplash
  fuguebot harvester fma
  fuguebot pioneer unsplash.com`,
}

var pioneerCmd = &cobra.Command{
	Use:   "pioneer <site>",
	Short: "Run Pioneer crawler for a site",
	Long:  "Pioneer explores sites and generates parsing scripts using AI.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		siteName := args[0]

		log.Printf("fuguebot: starting pioneer for site: %s", siteName)

		// Resolve site name to domain
		domain, err := resolveDomain(siteName)
		if err != nil {
			return err
		}
		log.Printf("fuguebot: resolved %s → %s", siteName, domain)

		// Initialize infrastructure
		infra, err := initInfrastructure()
		if err != nil {
			return fmt.Errorf("infrastructure initialization failed: %w", err)
		}
		defer infra.Close()

		// Get site from database
		ctx := context.Background()
		siteRepo := bot.NewSiteRepo(infra.DB)
		site, err := siteRepo.GetByDomain(ctx, domain)
		if err != nil {
			return fmt.Errorf("site not found in database: %s (domain: %s)", siteName, domain)
		}

		log.Printf("fuguebot: found site: %s (id: %s)", domain, site.ID)

		// Pioneer is now a scheduler-backed consumer (pioneer-scheduler-consumer).
		// The CLI seeds the site's root URL into pioneer_frontier and then
		// hands control to PioneerConsumer.Run for the lifetime of the process.
		return runPioneerConsumer(cmd.Context(), infra, site.RootUrl)
	},
}

var harvesterCmd = &cobra.Command{
	Use:   "harvester",
	Short: "Run Harvester consumer worker",
	Long: `Harvester is a URLScheduler consumer: it dequeues URLs from
harvester_frontier, fetches each via snapshot-first CompositeFetcher,
extracts a PinDocument, creates Pins, and reports back via SetStatus.
Takes no site argument — one worker processes URLs across all hosts in
priority order defined by harvester_frontier's partial index.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Println("fuguebot: starting harvester consumer worker")

		// Initialize infrastructure
		infra, err := initInfrastructure()
		if err != nil {
			return fmt.Errorf("infrastructure initialization failed: %w", err)
		}
		defer infra.Close()

		// Choose executor and pipeline based on HARVESTER_MODE. The executor
		// is consumed only by ScriptAdapter registration; mock executor is
		// still useful for exercising the consumer without a real goja runtime.
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		siteRepo := bot.NewSiteRepo(infra.DB)
		scriptRepo := bot.NewScriptRepo(infra.DB)

		var executor bot.ScriptExecutor
		var pipeline bot.DocumentPipeline

		mode := os.Getenv("HARVESTER_MODE")
		if mode == "real" {
			timeoutMs := 0 // 0 → GojaExecutor uses default 10000ms
			if v := os.Getenv("SCRIPT_TIMEOUT_MS"); v != "" {
				if parsed, parseErr := strconv.Atoi(v); parseErr == nil {
					timeoutMs = parsed
				}
			}
			executor = bot.NewGojaExecutor(timeoutMs)
			storageAdapter := bot.NewStorageAdapter(infra.Storage)
			pipeline = bot.NewHarvestPipeline(infra.Queries, storageAdapter)
			log.Println("fuguebot: using real executor + pipeline")
		} else {
			executor = bot.NewMockScriptExecutor()
			pipeline = bot.NewMockPipeline()
			log.Println("fuguebot: using mock executor + pipeline (set HARVESTER_MODE=real for production)")
		}

		// Apply env overrides for BotCreatorID (IMMUTABLE-sync policy
		// applies — see source.go doc comment).
		bot.ApplyBotCreatorIDFromEnv()

		// Build the AdapterRegistry and register a ScriptAdapter for every
		// active site that has at least one (site_id, node_type) script in
		// the DB. Sites without scripts fall through to the generic
		// extractor.
		scriptSites := map[uuid.UUID]bool{}
		if keys, keysErr := infra.Queries.ListScriptKeysForGraph(ctx); keysErr == nil {
			for _, k := range keys {
				scriptSites[k.SiteID] = true
			}
		} else {
			log.Printf("fuguebot: list script keys: %v (continuing with generic only)", keysErr)
		}
		registry := bot.NewInMemoryAdapterRegistry()
		if regErr := bot.RegisterScriptAdaptersForActiveSites(
			ctx, registry, siteRepo, scriptRepo, executor,
			func(_ context.Context, siteID uuid.UUID) (bool, error) {
				return scriptSites[siteID], nil
			},
		); regErr != nil {
			log.Printf("fuguebot: register script adapters: %v (continuing with generic only)", regErr)
		}

		// Snapshot-first Fetcher wiring (harvester-snapshot-first-fetch).
		// CompositeFetcher tries the ObjectStorage snapshot first and falls
		// back to HTTP on ANY error. The bucket must match the one Pioneer
		// writes to so keys line up bit-for-bit.
		harvestBucket := envOrDefault("PIONEER_SNAPSHOT_BUCKET", envOrDefault("S3_BUCKET", "fugue-media"))
		snapshotReader := snapshot.NewS3Reader(infra.Storage.S3Client(), harvestBucket)
		compositeFetcher := bot.NewCompositeFetcher(
			bot.NewObjectStorageFetcher(snapshotReader),
			bot.NewHTTPFetcher(),
		)

		// Scheduler boundary: URLs come from harvester_frontier via
		// Dequeue(QueueHarvester). FOR UPDATE SKIP LOCKED in the scheduler
		// guarantees no two consumer workers claim the same row even when
		// this command runs N times concurrently.
		sched := scheduler.NewPGURLScheduler(infra.DB).
			WithRateLimiter(scheduler.NewHostRateLimiter(scheduler.FactoryDefaultRatePerSec, scheduler.FactoryDefaultBurst, true))

		consumer := bot.NewHarvesterConsumer(
			sched,
			compositeFetcher,
			registry,
			bot.NewGenericExtractorFromEnv(),
			bot.NewClassifierFromEnv(),
			pipeline,
		)

		// Wire SIGINT/SIGTERM for clean worker shutdown between URLs.
		// URLScheduler.Dequeue does not take a context, so a pending Dequeue
		// may block up to one internal poll interval after cancellation
		// before Run observes it — documented in postgres_scheduler.go.
		runCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return consumer.Run(runCtx)
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge <site>",
	Short: "Merge duplicate URL-pattern nodes using Drain analysis",
	Long:  "Analyzes crawled nodes for a site using Drain algorithm, detects parameterized URL patterns, and merges them into {param} template nodes.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		siteName := args[0]
		threshold, _ := cmd.Flags().GetInt("threshold")

		domain, err := resolveDomain(siteName)
		if err != nil {
			return err
		}

		infra, err := initInfrastructure()
		if err != nil {
			return fmt.Errorf("infrastructure initialization failed: %w", err)
		}
		defer infra.Close()

		ctx := context.Background()
		site, err := infra.Queries.GetSiteByDomain(ctx, domain)
		if err != nil {
			return fmt.Errorf("site not found: %s (domain: %s)", siteName, domain)
		}

		log.Printf("fuguebot: running drain merge for %s (threshold: %d)...", domain, threshold)

		result, err := bot.RunDrainMerge(ctx, infra.Queries, site.ID, threshold)
		if err != nil {
			return fmt.Errorf("drain merge failed: %w", err)
		}

		if result.MergedPrefixes > 0 {
			log.Printf("fuguebot: merged %d prefixes, removed %d nodes", result.MergedPrefixes, result.RemovedNodes)
		} else {
			log.Println("fuguebot: no merge targets found")
		}

		return nil
	},
}

func init() {
	mergeCmd.Flags().Int("threshold", bot.DefaultMergeThreshold, "minimum leaf count to trigger merge")
	rootCmd.AddCommand(pioneerCmd)
	rootCmd.AddCommand(harvesterCmd)
	rootCmd.AddCommand(mergeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("fuguebot: %v", err)
	}
}

// runPioneerConsumer bootstraps the new scheduler-backed Pioneer loop.
// Seeds `seedURL` into pioneer_frontier (idempotent via ON CONFLICT DO NOTHING
// inside EnqueuePioneer) and then blocks in PioneerConsumer.Run. Returns when
// the consumer's context is cancelled or Run returns a fatal error.
//
// Wiring is intentionally minimal: the consumer has no notion of "sites", so
// the CLI arg (site name) only selects which root URL to seed. Subsequent
// links discovered via fanout B populate pioneer_frontier organically.
func runPioneerConsumer(parent context.Context, infra *Infrastructure, seedURL string) error {
	sched := scheduler.NewPGURLScheduler(infra.DB).
		WithRateLimiter(scheduler.NewHostRateLimiter(scheduler.FactoryDefaultRatePerSec, scheduler.FactoryDefaultBurst, true))

	snapshotBucket := envOrDefault("PIONEER_SNAPSHOT_BUCKET", envOrDefault("S3_BUCKET", "fugue-media"))
	store := snapshot.NewS3Store(infra.Storage.S3Client(), snapshotBucket)

	chain := bot.NewFilterChain(
		&bot.DomainFilter{},
		&bot.ExtensionFilter{},
		&bot.PathPatternFilter{},
		bot.NewRobotsFilter(nil),
		bot.NewCanonicalDedupFilter(nil),
	)

	consumer := bot.NewPioneerConsumer(sched, store, chain, bot.NewDefaultConsumerFetcher())

	if err := sched.Enqueue(scheduler.QueuePioneer, seedURL); err != nil {
		return fmt.Errorf("seed enqueue: %w", err)
	}
	log.Printf("fuguebot: seeded pioneer_frontier with %s", seedURL)

	// Wire SIGINT/SIGTERM so operators can stop the consumer cleanly between
	// URLs, and inherit the cobra command's parent context so ancestor
	// cancellation also propagates. Note: URLScheduler.Dequeue does not take
	// a context, so a pending Dequeue call may block up to one internal poll
	// interval before the cancellation is observed by the Run loop —
	// acknowledged in postgres_scheduler.go.
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return consumer.Run(ctx)
}
