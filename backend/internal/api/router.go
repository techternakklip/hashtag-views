package api

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type RouterOptions struct {
	AllowedOrigin   string
	FrontendDistDir string
	Logger          *slog.Logger
}

func NewRouter(h *Handler, opts RouterOptions) http.Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/rate-limits", h.RateLimits)
	mux.HandleFunc("GET /api/hashtags", h.ListHashtags)
	mux.HandleFunc("GET /api/hashtags/{tag}/summary", h.GetHashtagSummary)
	mux.HandleFunc("GET /api/hashtags/{tag}/videos", h.ListHashtagVideos)
	mux.HandleFunc("GET /api/hashtags/{tag}/export", h.ExportHashtag)
	mux.HandleFunc("POST /api/runs", h.CreateRun)
	mux.HandleFunc("GET /api/runs/{id}", h.GetRun)
	mux.HandleFunc("POST /api/tiktok/fetch", h.FetchTikTokHashtag)

	if opts.FrontendDistDir != "" && hasIndexHTML(opts.FrontendDistDir) {
		mux.Handle("/", spaFileHandler(opts.FrontendDistDir))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusNotFound, "endpoint tidak ditemukan")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{
				"service": "meta-hashtag-views API",
				"hint":    "Jalankan frontend Vite dev server (`cd frontend && npm run dev`) atau build ke frontend/dist",
			})
		})
	}

	var handler http.Handler = mux
	handler = withCORS(handler, opts.AllowedOrigin)
	handler = withSecurityHeaders(handler)
	handler = withRequestLogging(handler, logger)
	handler = withRecovery(handler, logger)
	return handler
}

func hasIndexHTML(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !info.IsDir()
}

func spaFileHandler(distDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(distDir))
	indexPath := filepath.Join(distDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "endpoint tidak ditemukan")
			return
		}

		cleanRel := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
		if cleanRel != "" && cleanRel != "." {
			candidate := filepath.Join(distDir, cleanRel)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, indexPath)
	})
}
