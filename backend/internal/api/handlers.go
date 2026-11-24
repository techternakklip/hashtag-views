package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"meta-hashtag-views/internal/domain"
	"meta-hashtag-views/internal/exporter"
	"meta-hashtag-views/internal/parser"
	"meta-hashtag-views/internal/ratelimit"
	"meta-hashtag-views/internal/service"
	"meta-hashtag-views/internal/store"
	"meta-hashtag-views/internal/tiktok"
)

const (
	maxRequestBodyBytes      = 1 << 20
	defaultTikTokCacheTTL    = 5 * time.Minute
	defaultTikTokMinInterval = 1200 * time.Millisecond
)

type Handler struct {
	repo              *store.Repository
	runner            *service.Runner
	tiktok            *tiktok.Client
	limiter           *ratelimit.Limiter
	enabledPlatforms  []domain.Platform
	enabledSet        map[domain.Platform]bool
	allowMockMode     bool
	tikTokCacheTTL    time.Duration
	tikTokMinInterval time.Duration
	nowFn             func() time.Time
	tikTokPaceMu      sync.Mutex
	lastTikTokReqAt   time.Time
}

func NewHandler(repo *store.Repository, runner *service.Runner) *Handler {
	enabled := append([]domain.Platform(nil), domain.AllPlatforms...)
	enabledSet := make(map[domain.Platform]bool, len(enabled))
	for _, p := range enabled {
		enabledSet[p] = true
	}
	return &Handler{
		repo:              repo,
		runner:            runner,
		tiktok:            tiktok.NewClient(""),
		limiter:           ratelimit.NewLimiter(nil),
		enabledPlatforms:  enabled,
		enabledSet:        enabledSet,
		allowMockMode:     true,
		tikTokCacheTTL:    defaultTikTokCacheTTL,
		tikTokMinInterval: defaultTikTokMinInterval,
		nowFn:             func() time.Time { return time.Now().UTC() },
	}
}

func (h *Handler) WithTikTokClient(c *tiktok.Client) *Handler {
	if c != nil {
		h.tiktok = c
	}
	return h
}

func (h *Handler) WithRateLimiter(l *ratelimit.Limiter) *Handler {
	if l != nil {
		h.limiter = l
	}
	return h
}

func (h *Handler) WithTikTokCacheTTL(d time.Duration) *Handler {
	if d < 0 {
		d = 0
	}
	h.tikTokCacheTTL = d
	return h
}

func (h *Handler) WithTikTokMinInterval(d time.Duration) *Handler {
	if d < 0 {
		d = 0
	}
	h.tikTokMinInterval = d
	return h
}

func (h *Handler) WithClock(fn func() time.Time) *Handler {
	if fn != nil {
		h.nowFn = fn
	}
	return h
}

func (h *Handler) WithEnabledPlatforms(platforms []string) *Handler {
	seen := make(map[domain.Platform]bool)
	var cleaned []domain.Platform
	for _, raw := range platforms {
		norm, err := parser.ParsePlatform(raw)
		if err != nil || seen[norm] {
			continue
		}
		seen[norm] = true
		cleaned = append(cleaned, norm)
	}
	if len(cleaned) == 0 {
		cleaned = []domain.Platform{domain.PlatformTikTok}
		seen = map[domain.Platform]bool{domain.PlatformTikTok: true}
	}
	h.enabledPlatforms = cleaned
	h.enabledSet = seen
	return h
}

func (h *Handler) WithAllowMockMode(allow bool) *Handler {
	h.allowMockMode = allow
	return h
}

func (h *Handler) isPlatformEnabled(p domain.Platform) bool {
	return h.enabledSet[p]
}

func (h *Handler) filteredRateLimits() map[domain.Platform]ratelimit.Status {
	all := h.limiter.AllStatuses()
	filtered := make(map[domain.Platform]ratelimit.Status, len(h.enabledPlatforms))
	for _, p := range h.enabledPlatforms {
		if st, ok := all[p]; ok {
			filtered[p] = st
		}
	}
	return filtered
}

func platformDisplayName(p domain.Platform) string {
	switch p {
	case domain.PlatformInstagram:
		return "Instagram"
	case domain.PlatformFacebook:
		return "Facebook"
	case domain.PlatformTikTok:
		return "TikTok"
	default:
		return string(p)
	}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"hasApifyToken":    h.runner.HasApifyToken(),
		"enabledPlatforms": h.enabledPlatforms,
		"allowMockMode":    h.allowMockMode,
		"metricFocus":      "real_video_views",
		"databaseEngine":   "sqlite",
		"serverTime":       time.Now().UTC().Format(time.RFC3339),
		"rateLimits":       h.filteredRateLimits(),
	})
}

func (h *Handler) RateLimits(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"serverTime": time.Now().UTC().Format(time.RFC3339),
		"limits":     h.filteredRateLimits(),
	})
}

func parseOptionalPlatformParam(raw string) (domain.Platform, error) {
	p := strings.ToLower(strings.TrimSpace(raw))
	if p == "" || p == "all" {
		return "", nil
	}
	return parser.ParsePlatform(p)
}

func (h *Handler) resolveRequestedPlatform(w http.ResponseWriter, raw string) (domain.Platform, bool) {
	platform, err := parseOptionalPlatformParam(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	if platform != "" && !h.isPlatformEnabled(platform) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("Platform %s belum tersedia.", platformDisplayName(platform)))
		return "", false
	}
	if platform == "" && len(h.enabledPlatforms) == 1 {
		platform = h.enabledPlatforms[0]
	}
	return platform, true
}

func (h *Handler) ListHashtags(w http.ResponseWriter, r *http.Request) {
	platform, ok := h.resolveRequestedPlatform(w, r.URL.Query().Get("platform"))
	if !ok {
		return
	}

	tags, err := h.repo.ListTrackedHashtagsByPlatform(r.Context(), platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal memuat daftar hashtag: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"platform": platform,
		"hashtags": tags,
	})
}

func (h *Handler) GetHashtagSummary(w http.ResponseWriter, r *http.Request) {
	tag, err := parser.NormalizeHashtag(r.PathValue("tag"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	platform, ok := h.resolveRequestedPlatform(w, r.URL.Query().Get("platform"))
	if !ok {
		return
	}

	summary, err := h.repo.GetHashtagSummaryByPlatform(r.Context(), tag, platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal memuat ringkasan #%s: %v", tag, err))
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) ListHashtagVideos(w http.ResponseWriter, r *http.Request) {
	tag, err := parser.NormalizeHashtag(r.PathValue("tag"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	q := r.URL.Query()
	platform, ok := h.resolveRequestedPlatform(w, q.Get("platform"))
	if !ok {
		return
	}

	limit := 500
	if rawLimit := strings.TrimSpace(q.Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "parameter limit harus bilangan positif")
			return
		}
		limit = parsed
	}

	videos, err := h.repo.ListHashtagVideos(r.Context(), tag, domain.VideoQueryOptions{
		Platform: string(platform),
		Search:   q.Get("q"),
		Sort:     strings.TrimSpace(q.Get("sort")),
		Limit:    limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal memuat video #%s: %v", tag, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hashtag": tag,
		"count":   len(videos),
		"videos":  videos,
	})
}

func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var opts domain.RunOptions
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("payload JSON tidak valid: %v", err))
		return
	}

	if len(opts.Platforms) > 0 {
		for _, p := range opts.Platforms {
			normPlat, err := parser.ParsePlatform(string(p))
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if !h.isPlatformEnabled(normPlat) {
				writeError(w, http.StatusForbidden, fmt.Sprintf("Platform %s belum tersedia.", platformDisplayName(normPlat)))
				return
			}
		}
	} else {
		var enabledMeta []domain.Platform
		for _, p := range h.enabledPlatforms {
			if p != domain.PlatformTikTok {
				enabledMeta = append(enabledMeta, p)
			}
		}
		if len(enabledMeta) == 0 {
			writeError(w, http.StatusForbidden, "Platform Instagram dan Facebook belum tersedia.")
			return
		}
		opts.Platforms = enabledMeta
	}

	if opts.Mode == domain.ModeMock && !h.allowMockMode {
		writeError(w, http.StatusForbidden, "Mode simulasi dinonaktifkan di server ini.")
		return
	}

	norm, err := h.runner.ValidateAndNormalizeOptions(opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if norm.Mode == domain.ModeLive {
		for _, plat := range norm.Platforms {
			allowed, rl := h.limiter.CheckAndConsume(plat)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfterSeconds))
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error":     fmt.Sprintf("Batas pencarian Live %s tercapai. Tunggu %s sebelum mencari lagi.", plat, ratelimit.FormatDurationID(rl.RetryAfterSeconds)),
					"rateLimit": rl,
				})
				return
			}
		}
	}

	rec, err := h.runner.StartAsync(norm)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, rec)
}

func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "run id tidak valid")
		return
	}

	rec, err := h.repo.GetRunByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "run tidak ditemukan")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal membaca status run: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

type tikTokFetchRequest struct {
	Hashtag string `json:"hashtag"`
	Force   bool   `json:"force,omitempty"`
}

func parseSnapshotTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func (h *Handler) waitTikTokPacing(ctx context.Context) error {
	if h.tikTokMinInterval <= 0 {
		return nil
	}
	h.tikTokPaceMu.Lock()
	now := h.nowFn()
	target := h.lastTikTokReqAt.Add(h.tikTokMinInterval)
	var wait time.Duration
	if !h.lastTikTokReqAt.IsZero() && target.After(now) {
		wait = target.Sub(now)
		h.lastTikTokReqAt = target
	} else {
		h.lastTikTokReqAt = now
	}
	h.tikTokPaceMu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Handler) FetchTikTokHashtag(w http.ResponseWriter, r *http.Request) {
	if !h.isPlatformEnabled(domain.PlatformTikTok) {
		writeError(w, http.StatusForbidden, "Platform TikTok belum tersedia.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var req tikTokFetchRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("payload JSON tidak valid: %v", err))
		return
	}

	tag, err := parser.NormalizeHashtag(req.Hashtag)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !req.Force && h.tikTokCacheTTL > 0 {
		if snaps, snapErr := h.repo.ListAggregateSnapshots(r.Context(), domain.PlatformTikTok, tag, 1); snapErr == nil && len(snaps) == 1 {
			if capturedAt, pErr := parseSnapshotTime(snaps[0].CapturedAt); pErr == nil {
				age := h.nowFn().Sub(capturedAt)
				if age >= -30*time.Second && age < h.tikTokCacheTTL {
					if summary, sumErr := h.repo.GetHashtagSummaryByPlatform(r.Context(), tag, domain.PlatformTikTok); sumErr == nil {
						writeJSON(w, http.StatusOK, map[string]any{
							"hashtag":        tag,
							"summary":        summary,
							"rateLimit":      h.limiter.Status(domain.PlatformTikTok),
							"cached":         true,
							"cacheExpiresAt": capturedAt.Add(h.tikTokCacheTTL).UTC().Format(time.RFC3339),
						})
						return
					}
				}
			}
		}
	}

	allowed, rl := h.limiter.CheckAndConsume(domain.PlatformTikTok)
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfterSeconds))
		msg := fmt.Sprintf("Batas pencarian TikTok tercapai. Tunggu %s sebelum mencari lagi.", ratelimit.FormatDurationID(rl.RetryAfterSeconds))
		if rl.UpstreamBlocked {
			msg = fmt.Sprintf("TikTok sedang membatasi permintaan. Cooldown %s sebelum mencoba lagi.", ratelimit.FormatDurationID(rl.RetryAfterSeconds))
		}
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":     msg,
			"rateLimit": rl,
		})
		return
	}

	if err := h.waitTikTokPacing(r.Context()); err != nil {
		writeError(w, http.StatusRequestTimeout, "permintaan dibatalkan")
		return
	}

	parsed, err := h.tiktok.FetchChallengeDetail(r.Context(), tag)
	if err != nil {
		switch {
		case errors.Is(err, parser.ErrTikTokHashtagNotFound):
			writeJSON(w, http.StatusNotFound, map[string]any{
				"error":     err.Error(),
				"rateLimit": h.limiter.Status(domain.PlatformTikTok),
			})
		case errors.Is(err, tiktok.ErrRateLimited):
			var rlErr *tiktok.RateLimitError
			var retryAfter time.Duration
			if errors.As(err, &rlErr) {
				retryAfter = rlErr.RetryAfter
			}
			cooldown := h.limiter.RecordUpstreamCooldown(domain.PlatformTikTok, retryAfter)
			w.Header().Set("Retry-After", strconv.Itoa(cooldown.RetryAfterSeconds))
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":     fmt.Sprintf("TikTok sedang membatasi permintaan. Cooldown %s sebelum mencoba lagi.", ratelimit.FormatDurationID(cooldown.RetryAfterSeconds)),
				"rateLimit": cooldown,
			})
		default:
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error":     err.Error(),
				"rateLimit": h.limiter.Status(domain.PlatformTikTok),
			})
		}
		return
	}

	resolvedTag := parsed.Hashtag
	if resolvedTag == "" {
		resolvedTag = tag
	}

	if _, err := h.repo.SaveAggregateSnapshot(
		r.Context(),
		domain.PlatformTikTok,
		resolvedTag,
		parsed.TotalVideos,
		parsed.TotalViews,
		"tiktok_challenge_detail",
		parsed.CapturedAt,
	); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal menyimpan snapshot TikTok: %v", err))
		return
	}

	summary, err := h.repo.GetHashtagSummaryByPlatform(r.Context(), resolvedTag, domain.PlatformTikTok)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal memuat ringkasan TikTok #%s: %v", resolvedTag, err))
		return
	}

	resp := map[string]any{
		"hashtag":   resolvedTag,
		"summary":   summary,
		"rateLimit": h.limiter.Status(domain.PlatformTikTok),
		"cached":    false,
	}
	if h.tikTokCacheTTL > 0 {
		resp["cacheExpiresAt"] = parsed.CapturedAt.Add(h.tikTokCacheTTL).UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ExportHashtag(w http.ResponseWriter, r *http.Request) {
	tag, err := parser.NormalizeHashtag(r.PathValue("tag"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		writeError(w, http.StatusBadRequest, "format export harus 'csv' atau 'json'")
		return
	}

	platform, ok := h.resolveRequestedPlatform(w, r.URL.Query().Get("platform"))
	if !ok {
		return
	}

	fileSuffix := ""
	if platform != "" {
		fileSuffix = "-" + string(platform)
	}

	if platform == domain.PlatformTikTok && format == "csv" {
		snapshots, err := h.repo.ListAggregateSnapshots(r.Context(), domain.PlatformTikTok, tag, 500)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal mengambil riwayat TikTok: %v", err))
			return
		}
		payload, err := exporter.ToAggregateCSV(snapshots)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal membuat CSV: %v", err))
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hashtag-%s%s-views.csv"`, tag, fileSuffix))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}

	var videos []domain.VideoViewRow
	if platform != domain.PlatformTikTok {
		videos, err = h.repo.ListHashtagVideos(r.Context(), tag, domain.VideoQueryOptions{
			Platform: string(platform),
			Sort:     "views_desc",
			Limit:    2000,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal mengambil video untuk export: %v", err))
			return
		}
	}

	if format == "csv" {
		payload, err := exporter.ToCSV(videos)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal membuat CSV: %v", err))
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hashtag-%s%s-views.csv"`, tag, fileSuffix))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}

	summary, err := h.repo.GetHashtagSummaryByPlatform(r.Context(), tag, platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal memuat ringkasan: %v", err))
		return
	}
	payload, err := exporter.ToJSONReport(summary, videos)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("gagal membuat JSON: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hashtag-%s%s-views.json"`, tag, fileSuffix))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
