package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	DefaultHost             = "127.0.0.1"
	DefaultPort             = "8080"
	DefaultDBRelPath        = "./data/hashtag-views.db"
	DefaultFrontendOrigin   = "http://localhost:5173"
	DefaultIGHashtagActor   = "apify/instagram-hashtag-scraper"
	DefaultFBHashtagActor   = "apify/facebook-hashtag-scraper"
	DefaultFBReelsActor     = "apify/facebook-reels-scraper"
	DefaultEnabledPlatforms = "instagram,facebook,tiktok"
)

type Config struct {
	ProjectRoot      string
	Host             string
	Port             string
	DBPath           string
	FrontendOrigin   string
	ApifyAPIToken    string
	IGHashtagActor   string
	FBHashtagActor   string
	FBReelsActor     string
	FrontendDistDir  string
	EnabledPlatforms []string
	AllowMockMode    bool
}

func Load() Config {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	root := detectProjectRoot(cwd)
	rootEnv := filepath.Join(root, ".env")
	_ = godotenv.Load(rootEnv)
	cwdEnv := filepath.Join(cwd, ".env")
	if cwdEnv != rootEnv {
		_ = godotenv.Load(cwdEnv)
	}

	dbPath := getEnvOrDefault("DB_PATH", DefaultDBRelPath)
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(root, dbPath)
	}

	return Config{
		ProjectRoot:     root,
		Host:            normalizeHost(os.Getenv("HOST")),
		Port:            validatePort(getEnvOrDefault("PORT", DefaultPort)),
		DBPath:          filepath.Clean(dbPath),
		FrontendOrigin:  getEnvOrDefault("FRONTEND_ORIGIN", DefaultFrontendOrigin),
		ApifyAPIToken:   os.Getenv("APIFY_API_TOKEN"),
		IGHashtagActor:  getEnvOrDefault("APIFY_IG_HASHTAG_ACTOR", DefaultIGHashtagActor),
		FBHashtagActor:  getEnvOrDefault("APIFY_FB_HASHTAG_ACTOR", DefaultFBHashtagActor),
		FBReelsActor:     getEnvOrDefault("APIFY_FB_REELS_ACTOR", DefaultFBReelsActor),
		FrontendDistDir:  filepath.Join(root, "frontend", "dist"),
		EnabledPlatforms: parseEnabledPlatforms(getEnvOrDefault("ENABLED_PLATFORMS", DefaultEnabledPlatforms)),
		AllowMockMode:    parseBoolOrDefault("ALLOW_MOCK_MODE", true),
	}
}

func detectProjectRoot(cwd string) string {
	if filepath.Base(cwd) == "backend" {
		parent := filepath.Dir(cwd)
		if exists(filepath.Join(parent, "frontend")) || exists(filepath.Join(parent, ".env.example")) {
			return parent
		}
	}
	return cwd
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func normalizeHost(raw string) string {
	if h := strings.TrimSpace(raw); h != "" {
		return h
	}
	return DefaultHost
}

func validatePort(raw string) string {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 65535 {
		return DefaultPort
	}
	return raw
}

func parseEnabledPlatforms(raw string) []string {
	aliases := map[string]string{
		"ig":        "instagram",
		"instagram": "instagram",
		"fb":        "facebook",
		"facebook":  "facebook",
		"tt":        "tiktok",
		"tiktok":    "tiktok",
	}
	seen := make(map[string]bool)
	var out []string
	for _, part := range strings.Split(raw, ",") {
		canonical, ok := aliases[strings.ToLower(strings.TrimSpace(part))]
		if ok && !seen[canonical] {
			seen[canonical] = true
			out = append(out, canonical)
		}
	}
	if len(out) == 0 {
		return []string{"tiktok"}
	}
	return out
}

func parseBoolOrDefault(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}