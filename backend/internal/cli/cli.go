package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"meta-hashtag-views/internal/api"
	"meta-hashtag-views/internal/apify"
	"meta-hashtag-views/internal/config"
	"meta-hashtag-views/internal/domain"
	"meta-hashtag-views/internal/exporter"
	"meta-hashtag-views/internal/parser"
	"meta-hashtag-views/internal/service"
	"meta-hashtag-views/internal/store"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	cmd := args[0]
	rest := args[1:]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		printUsage(stdout)
		return nil
	}

	cfg := config.Load()
	db, err := store.OpenSQLite(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	repo := store.NewRepository(db)
	apifyClient := apify.NewClient(cfg.ApifyAPIToken)
	runner := service.NewRunner(repo, apifyClient, cfg)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = runner.Shutdown(shutdownCtx)
	}()

	switch cmd {
	case "serve":
		return runServe(cfg, repo, runner, rest, stdout, stderr)
	case "fetch":
		return runFetch(runner, rest, stdout, stderr)
	case "report":
		return runReport(repo, rest, stdout, stderr)
	case "export":
		return runExport(repo, rest, stdout, stderr)
	default:
		printUsage(stderr)
		return fmt.Errorf("subcommand tidak dikenal: %q", cmd)
	}
}

func runServe(cfg config.Config, repo *store.Repository, runner *service.Runner, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.String("port", cfg.Port, "HTTP server port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	handler := api.NewHandler(repo, runner).
		WithEnabledPlatforms(cfg.EnabledPlatforms).
		WithAllowMockMode(cfg.AllowMockMode)
	router := api.NewRouter(handler, api.RouterOptions{
		AllowedOrigin:   cfg.FrontendOrigin,
		FrontendDistDir: cfg.FrontendDistDir,
		Logger:          logger,
	})

	srv := &http.Server{
		Addr:              ":" + *port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("Meta Hashtag Views API listening",
			"addr", "http://localhost:"+*port,
			"db_path", cfg.DBPath,
			"apify_configured", cfg.ApifyAPIToken != "",
			"enabled_platforms", cfg.EnabledPlatforms,
			"allow_mock_mode", cfg.AllowMockMode,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func runFetch(runner *service.Runner, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	hashtag := fs.String("hashtag", "", "Hashtag target (contoh: gta atau #gta)")
	platformsRaw := fs.String("platforms", "ig,fb", "Platform target: ig, fb, atau ig,fb")
	limit := fs.Int("limit", 50, "Limit item per platform")
	mock := fs.Bool("mock", false, "Jalankan mode simulasi tanpa kredit Apify")
	enrichFB := fs.Bool("enrich-fb", false, "Enrich video FB yang views-nya kosong dengan FB Reels Scraper")
	igType := fs.String("ig-type", "reels", "resultsType Instagram: reels atau posts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	platforms, err := parser.ParsePlatforms(*platformsRaw)
	if err != nil {
		return err
	}

	mode := domain.ModeLive
	if *mock {
		mode = domain.ModeMock
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(stdout, "Menarik data video #%s (platforms=%s, limit=%d, mode=%s)...\n",
		strings.TrimLeft(strings.TrimSpace(*hashtag), "#"), *platformsRaw, *limit, mode)

	rec, summary, err := runner.RunSync(ctx, domain.RunOptions{
		Hashtag:       *hashtag,
		Platforms:     platforms,
		Limit:         *limit,
		Mode:          mode,
		EnrichFB:      *enrichFB,
		IGResultsType: *igType,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Run #%d selesai dengan status: %s\n", rec.ID, rec.Status)
	if rec.ErrorMessage != "" {
		fmt.Fprintf(stdout, "Catatan / Peringatan: %s\n", rec.ErrorMessage)
	}
	printSummary(stdout, summary)
	return nil
}

func runReport(repo *store.Repository, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	hashtag := fs.String("hashtag", "", "Hashtag yang ingin dilaporkan")
	if err := fs.Parse(args); err != nil {
		return err
	}

	tag, err := parser.NormalizeHashtag(*hashtag)
	if err != nil {
		return err
	}

	summary, err := repo.GetHashtagSummary(context.Background(), tag)
	if err != nil {
		return err
	}
	printSummary(stdout, summary)

	videos, err := repo.ListHashtagVideos(context.Background(), tag, domain.VideoQueryOptions{
		Sort:  "views_desc",
		Limit: 10,
	})
	if err != nil {
		return err
	}
	printTopVideos(stdout, videos)
	return nil
}

func runExport(repo *store.Repository, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	hashtag := fs.String("hashtag", "", "Hashtag target export")
	format := fs.String("format", "csv", "Format export: csv atau json")
	outDir := fs.String("out", "./exports", "Direktori output file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	tag, err := parser.NormalizeHashtag(*hashtag)
	if err != nil {
		return err
	}

	fmtNorm := strings.ToLower(strings.TrimSpace(*format))
	if fmtNorm != "csv" && fmtNorm != "json" {
		return fmt.Errorf("format tidak valid: %q (gunakan csv atau json)", *format)
	}

	ctx := context.Background()
	videos, err := repo.ListHashtagVideos(ctx, tag, domain.VideoQueryOptions{
		Sort:  "views_desc",
		Limit: 2000,
	})
	if err != nil {
		return err
	}

	var payload []byte
	if fmtNorm == "csv" {
		payload, err = exporter.ToCSV(videos)
	} else {
		summary, sErr := repo.GetHashtagSummary(ctx, tag)
		if sErr != nil {
			return sErr
		}
		payload, err = exporter.ToJSONReport(summary, videos)
	}
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	fileName := fmt.Sprintf("hashtag-%s-views.%s", tag, fmtNorm)
	targetPath := filepath.Join(*outDir, fileName)
	if err := os.WriteFile(targetPath, payload, 0o644); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Export berhasil (%d video): %s\n", len(videos), targetPath)
	return nil
}
