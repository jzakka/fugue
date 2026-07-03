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
	"github.com/chungsanghwa/fugue/apps/api/internal/config"
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
  fuguebot harvester
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
		//
		// The mode must be chosen explicitly. Mock mode fabricates pin IDs
		// (uuid.New() with no pins row) that the consumer persists via
		// SetStatus(harvested, pinIDs), permanently consuming frontier rows
		// with zero pins created — so silently defaulting to mock on a
		// missing/typoed env var would corrupt scheduler state while logs
		// report successful "created" counts. Fail fast instead.
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		siteRepo := bot.NewSiteRepo(infra.DB)
		scriptRepo := bot.NewScriptRepo(infra.DB)

		var executor bot.ScriptExecutor
		var pipeline bot.DocumentPipeline

		switch mode := os.Getenv("HARVESTER_MODE"); mode {
		case "real":
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
		case "mock":
			executor = bot.NewMockScriptExecutor()
			pipeline = bot.NewMockPipeline()
			log.Println("fuguebot: using mock executor + pipeline (dev only — fabricated pin IDs consume frontier rows)")
		default:
			return fmt.Errorf("HARVESTER_MODE must be \"real\" or \"mock\", got %q", mode)
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

		// Snapshot-first fetch entry point wiring (harvester spec L563-657).
		// SnapshotFirstFetcher owns both the ObjectStorage and HTTP Fetcher
		// implementations so the consumer module never sees either type —
		// satisfying spec "consumer 모듈에 ObjectStorage/HTTP 클라이언트
		// 의존 부재". The bucket must match the one Pioneer writes to so
		// snapshot keys line up bit-for-bit.
		harvestBucket := envOrDefault("PIONEER_SNAPSHOT_BUCKET", envOrDefault("S3_BUCKET", "fugue-media"))
		snapshotReader := snapshot.NewS3Reader(infra.Storage.S3Client(), harvestBucket)
		snapshotFetcher := bot.NewSnapshotFirstFetcher(
			bot.NewObjectStorageFetcher(snapshotReader),
			bot.NewHTTPFetcher(),
		)

		// Scheduler boundary: URLs come from harvester_frontier via
		// Dequeue(QueueHarvester). FOR UPDATE SKIP LOCKED in the scheduler
		// guarantees no two consumer workers claim the same row even when
		// this command runs N times concurrently.
		sched := scheduler.NewPGURLScheduler(infra.DB).
			WithRateLimiter(buildHostRateLimiter(config.LoadSchedulerHostConfig()))

		consumer := buildHarvesterConsumer(
			sched,
			snapshotFetcher,
			registry,
			bot.NewGenericExtractorFromEnv(),
			bot.NewClassifierFromEnv(),
			pipeline,
		)

		// Wire SIGINT/SIGTERM for clean worker shutdown. The harvester Run
		// loop calls URLScheduler.DequeueCtx(runCtx, ...) so cancellation
		// propagates into the poll select and the tryClaim transaction;
		// SIGTERM unblocks Dequeue within a few ms instead of waiting up to
		// one pollInterval (~1s).
		runCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return consumer.Run(runCtx)
	},
}

// backfillPlaceholdersCmd implements tasks 6.2 / 6.3 of harvester-media-validation.
// It reads Pin rows whose media_url matches one of the legacy
// pre-deployment placeholder shapes documented in
// db/scripts/backfill_placeholder_media.sql Q1 (creator_id=BotCreatorID +
// media_url under the legacy `bot/` or `image/` prefix with a `.gif`
// extension) and re-queues each Pin's canonical URL into harvester_frontier
// so the new validator path replaces the placeholder with a real candidate
// (or routes the doc to no_primary_media).
//
// Operational contract:
//   - --dry-run reports the count + first few rows without enqueuing (task 6.3)
//   - Without --dry-run, each canonical URL is enqueued via URLScheduler.Enqueue.
//     Per-host rate limiting is enforced server-side at Dequeue time by the
//     HostRateLimiter wired into the scheduler; this command additionally
//     paces enqueue-side calls with a small sleep so a large backlog does not
//     overwhelm the frontier in a single transaction burst.
var backfillPlaceholdersCmd = &cobra.Command{
	Use:   "backfill-placeholders",
	Short: "Re-queue Pins whose media_url is a legacy placeholder GIF",
	Long: `Identifies Pins where media_url points to one of the legacy
placeholder shapes (under BotCreatorID, with media_url containing
either "/bot/" or "/image/" and ending in ".gif") and re-queues each
canonical URL into harvester_frontier so the new media validator path
replaces or rejects the candidate.

Use --dry-run first to see how many rows would be affected.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		pacingMs, _ := cmd.Flags().GetInt("pace-ms")

		infra, err := initInfrastructure()
		if err != nil {
			return fmt.Errorf("infrastructure initialization failed: %w", err)
		}
		defer infra.Close()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Apply env overrides for BotCreatorID before the SQL filter runs.
		bot.ApplyBotCreatorIDFromEnv()
		creatorID := bot.BotCreatorID

		// Q1 from db/scripts/backfill_placeholder_media.sql — kept inline so the
		// CLI does not depend on filesystem layout at runtime. The two ILIKE
		// predicates cover the QA-reported legacy single-segment "/image/" key
		// shape with a .gif extension.
		const selectPlaceholders = `
SELECT id::text, url, media_url
FROM pins
WHERE creator_id = $1
  AND (media_url ILIKE '%/bot/%' OR media_url ILIKE '%/image/%')
  AND media_url ILIKE '%.gif'
ORDER BY created_at DESC`

		rows, err := infra.DB.QueryContext(ctx, selectPlaceholders, creatorID)
		if err != nil {
			return fmt.Errorf("query placeholder pins: %w", err)
		}
		defer func() { _ = rows.Close() }()

		type pinRow struct {
			ID, URL, MediaURL string
		}
		var pins []pinRow
		for rows.Next() {
			var p pinRow
			if scanErr := rows.Scan(&p.ID, &p.URL, &p.MediaURL); scanErr != nil {
				return fmt.Errorf("scan placeholder pin: %w", scanErr)
			}
			pins = append(pins, p)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return fmt.Errorf("iterate placeholder pins: %w", rowsErr)
		}

		log.Printf("fuguebot: backfill-placeholders: %d pin(s) match placeholder pattern", len(pins))

		// Show a small sample so the operator can sanity-check the pattern
		// before approving the non-dry-run pass.
		preview := len(pins)
		if preview > 5 {
			preview = 5
		}
		for i := 0; i < preview; i++ {
			log.Printf("  sample[%d]: pin=%s url=%s media_url=%s", i, pins[i].ID, pins[i].URL, pins[i].MediaURL)
		}

		if dryRun {
			log.Printf("fuguebot: --dry-run set; not enqueuing. Re-run without --dry-run after operator approval.")
			return nil
		}

		if len(pins) == 0 {
			return nil
		}

		sched := scheduler.NewPGURLScheduler(infra.DB)
		enqueued := 0
		var failures []string
		for _, p := range pins {
			if ctx.Err() != nil {
				log.Printf("fuguebot: context cancelled after %d enqueue(s)", enqueued)
				break
			}
			if enqErr := sched.Enqueue(scheduler.QueueHarvester, p.URL); enqErr != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", p.ID, enqErr))
				continue
			}
			enqueued++
			if pacingMs > 0 {
				time.Sleep(time.Duration(pacingMs) * time.Millisecond)
			}
		}

		log.Printf("fuguebot: backfill-placeholders: enqueued %d/%d, failures=%d", enqueued, len(pins), len(failures))
		for _, f := range failures {
			log.Printf("  failure: %s", f)
		}
		return nil
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
	backfillPlaceholdersCmd.Flags().Bool("dry-run", false, "report match count without enqueuing")
	backfillPlaceholdersCmd.Flags().Int("pace-ms", 50, "sleep between enqueues in milliseconds (0 disables)")
	rootCmd.AddCommand(pioneerCmd)
	rootCmd.AddCommand(harvesterCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(backfillPlaceholdersCmd)
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
	rl := buildHostRateLimiter(config.LoadSchedulerHostConfig())
	sched := scheduler.NewPGURLScheduler(infra.DB).WithRateLimiter(rl)

	snapshotBucket := envOrDefault("PIONEER_SNAPSHOT_BUCKET", envOrDefault("S3_BUCKET", "fugue-media"))
	store := snapshot.NewS3Store(infra.Storage.S3Client(), snapshotBucket)

	consumer, _ := buildPioneerConsumer(sched, store, rl)

	if err := sched.Enqueue(scheduler.QueuePioneer, seedURL); err != nil {
		return fmt.Errorf("seed enqueue: %w", err)
	}
	log.Printf("fuguebot: seeded pioneer_frontier with %s", seedURL)

	// Wire SIGINT/SIGTERM so operators can stop the consumer cleanly between
	// URLs, and inherit the cobra command's parent context so ancestor
	// cancellation also propagates. The pioneer consumer Run loop calls
	// URLScheduler.DequeueCtx(ctx, ...) so cancellation propagates into the
	// poll select and the tryClaim transaction; SIGTERM unblocks Dequeue
	// within a few ms instead of waiting up to one pollInterval (~1s).
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return consumer.Run(ctx)
}
