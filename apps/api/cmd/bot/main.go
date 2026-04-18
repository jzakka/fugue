package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/ai"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
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

// envBool parses a boolean env var (1/true/t/yes/y vs 0/false/f/no/n,
// case-insensitive). Unset or unparseable falls back to the supplied
// default so the bot stays safe when config is missing.
func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
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

		// Initialize Pioneer dependencies
		graphRepo := bot.NewGraphRepo(infra.DB)
		scriptRepo := bot.NewScriptRepo(infra.DB)

		// Initialize AI client (CLI mode by default, SDK mode with AI_CLIENT_TYPE=sdk)
		rawAIClient, err := ai.NewFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create AI client: %w", err)
		}

		// Wrap with adapter to implement bot.AIClient interface
		aiClient := bot.NewAIClientAdapter(rawAIClient)

		// Initialize script executor (GojaExecutor for real script validation)
		executor := bot.NewGojaExecutor(0)

		// Create Pioneer instance
		pioneer := bot.NewPioneer(
			siteRepo,
			graphRepo,
			scriptRepo,
			aiClient,
			executor,
			bot.PioneerConfig{
				MaxNodesPerSite:  100,
				RateLimitMs:      500,
				SuccessThreshold: 0.7,
			},
		)

		// Wire snapshot saver based on PIONEER_SNAPSHOT_ENABLED
		// (openspec: pioneer-snapshot-storage). When false the Pioneer
		// keeps its default noop saver, so fetch/link-extraction still run
		// but no object-storage upload occurs.
		if envBool("PIONEER_SNAPSHOT_ENABLED", false) {
			snapStore := snapshot.NewS3Store(infra.Storage.S3Client(), infra.Storage.Bucket())
			snapSaver := snapshot.NewSaver(snapStore, snapshot.NewMetricsRecorder(), nil)
			pioneer = pioneer.WithSnapshotSaver(snapSaver)
			log.Println("fuguebot: pioneer snapshot upload enabled")
		}

		// Run Pioneer
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := pioneer.Run(ctx, site.ID); err != nil {
			log.Printf("fuguebot: pioneer failed: %v", err)
			return err
		}

		log.Println("fuguebot: pioneer run completed")

		// Post-crawl: drain merge to deduplicate parameterized URL nodes
		log.Println("fuguebot: running drain merge...")
		mergeResult, mergeErr := bot.RunDrainMerge(ctx, infra.Queries, site.ID, bot.DefaultMergeThreshold)
		if mergeErr != nil {
			log.Printf("fuguebot: drain merge failed: %v", mergeErr)
			return mergeErr
		}
		if mergeResult.MergedPrefixes > 0 {
			log.Printf("fuguebot: drain merge done — %d prefixes merged, %d nodes removed", mergeResult.MergedPrefixes, mergeResult.RemovedNodes)
		} else {
			log.Println("fuguebot: drain merge — no merge targets found")
		}

		return nil
	},
}

var harvesterCmd = &cobra.Command{
	Use:   "harvester <site>",
	Short: "Run Harvester crawler for a site",
	Long:  "Harvester executes scripts and extracts content from sites.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		siteName := args[0]

		log.Printf("fuguebot: starting harvester for site: %s", siteName)

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

		// Initialize Harvester dependencies
		graphRepo := bot.NewGraphRepo(infra.DB)
		scriptRepo := bot.NewScriptRepo(infra.DB)

		// Choose executor and pipeline based on HARVESTER_MODE
		var executor bot.ScriptExecutor
		var pipeline bot.Pipeline

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

		// Create Harvester instance
		harvester := bot.NewHarvester(
			siteRepo,
			graphRepo,
			scriptRepo,
			executor,
			pipeline,
			bot.HarvesterConfig{
				RateLimitMs:      500,
				RetryFailedNodes: false,
				MaxRetries:       3,
			},
		)

		// Run Harvester
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		stats, err := harvester.Run(ctx, site.ID)
		if err != nil {
			log.Printf("fuguebot: harvester failed: %v", err)
			return err
		}

		log.Printf("fuguebot: harvester completed — nodes: %d, pins created: %d, deduped: %d, failed: %d",
			stats.NodesProcessed, stats.PinsCreated, stats.Deduped, stats.Failed)
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
	rootCmd.AddCommand(pioneerCmd)
	rootCmd.AddCommand(harvesterCmd)
	rootCmd.AddCommand(mergeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("fuguebot: %v", err)
	}
}
