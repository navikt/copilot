package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        string
	Environment string
	// EnableDevQuery is the second half of the double-lock guarding the
	// unauthenticated raw-SQL /dev/query endpoint (see main.go). Environment
	// defaults to "local" when NAIS_CLUSTER_NAME is unset, so gating on the
	// environment alone would fail OPEN: a misconfigured deployment missing
	// that env var would silently expose an unauthenticated SQL console.
	// Requiring an explicit ENABLE_DEV_QUERY=true opt-in ensures the endpoint
	// can never appear just because an env var went missing.
	EnableDevQuery         bool
	LogLevel               slog.Level
	LoggedEndpoints        map[string]bool
	AzureClientID          string
	AzureIssuer            string
	AzureJWKSURI           string
	PreAuthorizedApps      string
	GitHubOrg              string
	GitHubEnterprise       string
	GitHubAppID            string
	GitHubAppPrivateKey    string
	GitHubInstallationID   string
	GitHubBillingToken     string // classic PAT, used only when BillingPATFallback is true
	BillingPATFallback     bool   // BILLING_PAT_FALLBACK=true: billing reads use the PAT (transition only)
	GCPProjectID           string
	CopilotMetricsDataset  string
	CopilotMetricsTable    string
	CopilotAdoptionDataset string
	CopilotMimirDataset    string
	CacheTTLHours          int
	VideoManifestURL       string
	VideoManifestPath      string
	VideoBucketPublic      string
	VideoPublicBaseURL     string
	VideoFeedCacheSeconds  int

	// Personal budget raises (PRD prd-budsjett-og-paafyll). See budget_raise.go.
	BudgetDatabaseURL       string
	BudgetWrites            string // off | dry_run | on; anything else is off
	BudgetAppID             string
	BudgetAppPrivateKey     string
	BudgetAppInstallationID string
	BudgetDefaultUSD        int
	BudgetGitHubBudgetType  string
	BudgetTimeZone          string
	BudgetRaisesOpen        bool
	BudgetTestUsers         map[string]bool
	BudgetReconcileInterval time.Duration
}

func Load() *Config {
	environment := getEnv("NAIS_CLUSTER_NAME", "local")
	config := &Config{
		Port:                   getEnv("PORT", "8080"),
		Environment:            getEnv("NAIS_CLUSTER_NAME", "local"),
		EnableDevQuery:         getEnv("ENABLE_DEV_QUERY", "") == "true",
		LogLevel:               parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		LoggedEndpoints:        parseEndpoints(getEnv("LOGGED_ENDPOINTS", "/api/v1/")),
		AzureClientID:          getEnv("AZURE_APP_CLIENT_ID", ""),
		AzureIssuer:            getEnv("AZURE_OPENID_CONFIG_ISSUER", ""),
		AzureJWKSURI:           getEnv("AZURE_OPENID_CONFIG_JWKS_URI", ""),
		PreAuthorizedApps:      getEnv("AZURE_APP_PRE_AUTHORIZED_APPS", ""),
		GitHubOrg:              getEnv("GITHUB_ORG", "navikt"),
		GitHubEnterprise:       getEnv("GITHUB_ENTERPRISE", "nav"),
		GitHubAppID:            os.Getenv("GITHUB_APP_ID"),
		GitHubAppPrivateKey:    os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		GitHubInstallationID:   os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		GitHubBillingToken:     os.Getenv("GITHUB_BILLING_TOKEN"),
		BillingPATFallback:     os.Getenv("BILLING_PAT_FALLBACK") == "true",
		GCPProjectID:           os.Getenv("GCP_TEAM_PROJECT_ID"),
		CopilotMetricsDataset:  getEnv("COPILOT_METRICS_DATASET", "copilot_metrics"),
		CopilotMetricsTable:    getEnv("COPILOT_METRICS_TABLE", "usage_metrics"),
		CopilotAdoptionDataset: getEnv("COPILOT_ADOPTION_DATASET", "copilot_adoption"),
		CopilotMimirDataset:    getEnv("COPILOT_MIMIR_DATASET", "copilot_metrics_staging"),
		CacheTTLHours:          1,
		VideoManifestURL:       getEnv("VIDEO_MANIFEST_URL", ""),
		VideoManifestPath:      getEnv("VIDEO_MANIFEST_PATH", ""),
		VideoBucketPublic:      selectVideoValue(environment, os.Getenv("VIDEO_BUCKET_PUBLIC_DEV"), os.Getenv("VIDEO_BUCKET_PUBLIC_PROD"), os.Getenv("VIDEO_BUCKET_PUBLIC")),
		VideoPublicBaseURL:     getEnv("VIDEO_PUBLIC_BASE_URL", ""),
		VideoFeedCacheSeconds:  getEnvInt("VIDEO_FEED_CACHE_SECONDS", 60),

		BudgetDatabaseURL:       os.Getenv("DB_URL"),
		BudgetWrites:            ParseBudgetWrites(os.Getenv("BUDGET_WRITES")),
		BudgetAppID:             os.Getenv("BUDGET_GITHUB_APP_ID"),
		BudgetAppPrivateKey:     os.Getenv("BUDGET_GITHUB_APP_PRIVATE_KEY"),
		BudgetAppInstallationID: os.Getenv("BUDGET_GITHUB_APP_INSTALLATION_ID"),
		BudgetDefaultUSD:        getEnvInt("BUDGET_DEFAULT_USD", 150),
		// Every live AI-credit budget on the enterprise lists BundlePricing; P1 verifies the write.
		BudgetGitHubBudgetType: getEnv("BUDGET_GITHUB_BUDGET_TYPE", "BundlePricing"),
		BudgetTimeZone:         getEnv("BUDGET_TIME_ZONE", "Europe/Oslo"),
		BudgetRaisesOpen:       os.Getenv("BUDGET_RAISES_OPEN") == "true",
		BudgetTestUsers:        parseLoginSet(os.Getenv("BUDGET_TEST_USERS")),
		// Settles raises stuck in pending (#2718); 0 or less falls back to 5 minutes.
		BudgetReconcileInterval: time.Duration(max(getEnvInt("BUDGET_RECONCILE_INTERVAL_SECONDS", 300), 0)) * time.Second,
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return n
}

func selectVideoValue(environment, devValue, prodValue, fallback string) string {
	switch normalizeEnvironment(environment) {
	case "dev":
		if devValue != "" {
			return devValue
		}
	case "prod":
		if prodValue != "" {
			return prodValue
		}
	}

	if fallback != "" {
		return fallback
	}
	if devValue != "" {
		return devValue
	}
	if prodValue != "" {
		return prodValue
	}
	return ""
}

func normalizeEnvironment(environment string) string {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "dev", "dev-gcp", "development":
		return "dev"
	case "prod", "prod-gcp", "production":
		return "prod"
	default:
		return ""
	}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseEndpoints(input string) map[string]bool {
	endpoints := make(map[string]bool)
	for _, endpoint := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(endpoint)
		if trimmed != "" {
			endpoints[trimmed] = true
		}
	}
	return endpoints
}

// ParseBudgetWrites maps BUDGET_WRITES to off, dry_run or on. Unknown is off.
func ParseBudgetWrites(v string) string {
	switch v {
	case "dry_run", "on":
		return v
	}
	return "off"
}

// parseLoginSet reads a comma-separated list of GitHub logins, lower-cased.
func parseLoginSet(v string) map[string]bool {
	set := map[string]bool{}
	for _, l := range strings.Split(v, ",") {
		if l = strings.ToLower(strings.TrimSpace(l)); l != "" {
			set[l] = true
		}
	}
	return set
}
