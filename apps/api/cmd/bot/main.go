package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/ai"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

// Source name to domain registry
var sourceRegistry = map[string]string{
	"unsplash": "unsplash.com",
	"fma":      "freemusicarchive.org",
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

		// Initialize Pioneer dependencies
		graphRepo := bot.NewGraphRepo(infra.DB)
		scriptRepo := bot.NewScriptRepo(infra.DB)

		// Initialize AI client
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable is required")
		}
		rawAIClient, err := ai.NewOpenAIClient(ai.Config{
			APIKey:  apiKey,
			Model:   envOrDefault("OPENAI_MODEL", "gpt-4o"),
			Timeout: 30 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("failed to create AI client: %w", err)
		}

		// Wrap with adapter to implement bot.AIClient interface
		aiClient := bot.NewAIClientAdapter(rawAIClient)

		// Initialize script executor (using mock for now as there's no real implementation)
		executor := bot.NewMockScriptExecutor()

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

		// Run Pioneer
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := pioneer.Run(ctx, site.ID); err != nil {
			log.Printf("fuguebot: pioneer failed: %v", err)
			return err
		}

		log.Println("fuguebot: pioneer run completed")
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

		// Initialize script executor (using mock for now)
		executor := bot.NewMockScriptExecutor()

		// Initialize pipeline (using mock for now as Pipeline is just an interface)
		pipeline := &bot.MockPipeline{}

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

		if err := harvester.Run(ctx, site.ID); err != nil {
			log.Printf("fuguebot: harvester failed: %v", err)
			return err
		}

		log.Println("fuguebot: harvester run completed")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pioneerCmd)
	rootCmd.AddCommand(harvesterCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("fuguebot: %v", err)
	}
}
