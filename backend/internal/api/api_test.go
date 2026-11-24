package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"meta-hashtag-views/internal/apify"
	"meta-hashtag-views/internal/config"
	"meta-hashtag-views/internal/domain"
	"meta-hashtag-views/internal/ratelimit"
	"meta-hashtag-views/internal/service"
	"meta-hashtag-views/internal/store"
	"meta-hashtag-views/internal/tiktok"
)

func TestAPIWorkflowMockRunSummaryVideosAndExport(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	repo := store.NewRepository(db)
	runner := service.NewRunner(repo, apify.NewClient(""), config.Config{})
	defer func() { _ = runner.Shutdown(context.Background()) }()

	router := NewRouter(NewHandler(repo, runner), RouterOptions{
		AllowedOrigin: "http://localhost:5173",
	})
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}

	reqBody := bytes.NewBufferString(`{"hashtag":"#GTA","platforms":["instagram","facebook"],"limit":10,"mode":"mock","enrichFb":false}`)
	postResp, err := http.Post(srv.URL+"/api/runs", "application/json", reqBody)
	if err != nil {
		t.Fatalf("POST /api/runs failed: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /api/runs status = %d, want 202", postResp.StatusCode)
	}
	var created domain.RunRecord
	if err := json.NewDecoder(postResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var finalRun domain.RunRecord
	for time.Now().Before(deadline) {
		r, err := http.Get(srv.URL + "/api/runs/" + strconv.FormatInt(created.ID, 10))
		if err != nil {
			t.Fatalf("GET /api/runs/{id} failed: %v", err)
		}
		_ = json.NewDecoder(r.Body).Decode(&finalRun)
		r.Body.Close()
		if finalRun.Status == domain.StatusCompleted || finalRun.Status == domain.StatusFailed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if finalRun.Status != domain.StatusCompleted {
		t.Fatalf("expected run completed, got %+v", finalRun)
	}

	sumResp, err := http.Get(srv.URL + "/api/hashtags/gta/summary")
	if err != nil {
		t.Fatalf("GET summary failed: %v", err)
	}
	defer sumResp.Body.Close()
	var summary domain.HashtagSummary
	if err := json.NewDecoder(sumResp.Body).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.GrandTotalViews <= 0 || summary.TotalVideos != 12 || summary.VideosMissingViews != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	vidResp, err := http.Get(srv.URL + "/api/hashtags/gta/videos?platform=instagram&sort=views_desc")
	if err != nil {
		t.Fatalf("GET videos failed: %v", err)
	}
	defer vidResp.Body.Close()
	var vidBody struct {
		Count  int                   `json:"count"`
		Videos []domain.VideoViewRow `json:"videos"`
	}
	if err := json.NewDecoder(vidResp.Body).Decode(&vidBody); err != nil {
		t.Fatalf("decode videos: %v", err)
	}
	if vidBody.Count != 6 {
		t.Fatalf("expected 6 IG videos, got %d", vidBody.Count)
	}

	csvResp, err := http.Get(srv.URL + "/api/hashtags/gta/export?format=csv")
	if err != nil {
		t.Fatalf("GET export csv failed: %v", err)
	}
	defer csvResp.Body.Close()
	if csvResp.StatusCode != http.StatusOK {
		t.Fatalf("export csv status = %d, want 200", csvResp.StatusCode)
	}
	var rawCSV bytes.Buffer
	_, _ = rawCSV.ReadFrom(csvResp.Body)
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(rawCSV.Bytes(), []byte("\xEF\xBB\xBF")))).ReadAll()
	if err != nil {
		t.Fatalf("parse exported csv: %v", err)
	}
	if len(rows) != 13 {
		t.Fatalf("expected 13 CSV rows (header + 12 videos), got %d", len(rows))
	}

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing expected security headers: %v", resp.Header)
	}
}

func TestMiddlewareRecoveryHandlesPanic(t *testing.T) {
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicking := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("simulated failure")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	withRecovery(panicking, silentLogger).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500", rr.Code)
	}
}

func TestAPITikTokDirectFetchSummaryAndExport(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "tiktok-api.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	upstreamTikTok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("challengeName") {
		case "kuliner":
			_, _ = w.Write([]byte(`{"statusCode":0,"extra":{"now":1795164626000},"challengeInfo":{"challenge":{"title":"kuliner"},"statsV2":{"videoCount":"4721643","viewCount":"84098188263"}}}`))
		case "tidakada":
			_, _ = w.Write([]byte(`{"statusCode":10205,"statusMsg":"challenge not found"}`))
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer upstreamTikTok.Close()

	repo := store.NewRepository(db)
	runner := service.NewRunner(repo, apify.NewClient(""), config.Config{})
	defer func() { _ = runner.Shutdown(context.Background()) }()

	handler := NewHandler(repo, runner).
		WithTikTokClient(tiktok.NewClient(upstreamTikTok.URL)).
		WithTikTokMinInterval(0)
	srv := httptest.NewServer(NewRouter(handler, RouterOptions{}))
	defer srv.Close()

	resp1, err := http.Post(srv.URL+"/api/tiktok/fetch", "application/json", bytes.NewBufferString(`{"hashtag":"#Kuliner"}`))
	if err != nil {
		t.Fatalf("POST /api/tiktok/fetch failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("fetch status = %d, want 200", resp1.StatusCode)
	}

	var fetched struct {
		Hashtag string                `json:"hashtag"`
		Summary domain.HashtagSummary `json:"summary"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode fetch response: %v", err)
	}
	if fetched.Hashtag != "kuliner" || !fetched.Summary.IsAggregateOnly || fetched.Summary.GrandTotalViews != 84098188263 || fetched.Summary.TotalVideos != 4721643 {
		t.Fatalf("unexpected fetch response: %+v", fetched)
	}

	respNF, err := http.Post(srv.URL+"/api/tiktok/fetch", "application/json", bytes.NewBufferString(`{"hashtag":"tidakada"}`))
	if err != nil {
		t.Fatalf("POST /api/tiktok/fetch (10205) failed: %v", err)
	}
	respNF.Body.Close()
	if respNF.StatusCode != http.StatusNotFound {
		t.Fatalf("statusCode 10205 HTTP status = %d, want 404", respNF.StatusCode)
	}

	runResp, err := http.Post(srv.URL+"/api/runs", "application/json", bytes.NewBufferString(`{"hashtag":"kuliner","platforms":["tiktok"],"limit":10,"mode":"mock"}`))
	if err != nil {
		t.Fatalf("POST /api/runs tiktok failed: %v", err)
	}
	runResp.Body.Close()
	if runResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /api/runs tiktok status = %d, want 400", runResp.StatusCode)
	}

	sumResp, err := http.Get(srv.URL + "/api/hashtags/kuliner/summary?platform=tiktok")
	if err != nil {
		t.Fatalf("GET tiktok summary failed: %v", err)
	}
	defer sumResp.Body.Close()
	var sum domain.HashtagSummary
	if err := json.NewDecoder(sumResp.Body).Decode(&sum); err != nil {
		t.Fatalf("decode tiktok summary: %v", err)
	}
	if !sum.IsAggregateOnly || sum.GrandTotalViews != 84098188263 {
		t.Fatalf("unexpected tiktok summary: %+v", sum)
	}

	csvResp, err := http.Get(srv.URL + "/api/hashtags/kuliner/export?format=csv&platform=tiktok")
	if err != nil {
		t.Fatalf("GET tiktok export csv failed: %v", err)
	}
	defer csvResp.Body.Close()
	if csvResp.StatusCode != http.StatusOK {
		t.Fatalf("tiktok export csv status = %d, want 200", csvResp.StatusCode)
	}
	var rawCSV bytes.Buffer
	_, _ = rawCSV.ReadFrom(csvResp.Body)
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(rawCSV.Bytes(), []byte("\xEF\xBB\xBF")))).ReadAll()
	if err != nil {
		t.Fatalf("parse tiktok csv: %v", err)
	}
	if len(rows) != 2 || rows[0][0] != "snapshot_id" || rows[1][4] != "84098188263" {
		t.Fatalf("unexpected tiktok csv rows: %+v", rows)
	}
}

func TestAPIRateLimitEnforcementAndCooldown(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "rl-api.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	upstreamTikTok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("challengeName") == "throttle" {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"statusCode":0,"extra":{"now":1795164626000},"challengeInfo":{"challenge":{"title":"kuliner"},"statsV2":{"videoCount":"100","viewCount":"5000"}}}`))
	}))
	defer upstreamTikTok.Close()

	now := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	lim := ratelimit.NewLimiter(map[domain.Platform]ratelimit.Policy{
		domain.PlatformTikTok: {
			Limit:           2,
			Window:          10 * time.Minute,
			DefaultCooldown: 5 * time.Minute,
		},
	}).WithClock(func() time.Time { return now })

	repo := store.NewRepository(db)
	runner := service.NewRunner(repo, apify.NewClient(""), config.Config{})
	defer func() { _ = runner.Shutdown(context.Background()) }()

	handler := NewHandler(repo, runner).
		WithTikTokClient(tiktok.NewClient(upstreamTikTok.URL)).
		WithRateLimiter(lim).
		WithTikTokMinInterval(0).
		WithTikTokCacheTTL(0)
	srv := httptest.NewServer(NewRouter(handler, RouterOptions{}))
	defer srv.Close()

	rlResp, err := http.Get(srv.URL + "/api/rate-limits")
	if err != nil {
		t.Fatalf("GET /api/rate-limits failed: %v", err)
	}
	defer rlResp.Body.Close()
	if rlResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/rate-limits status = %d, want 200", rlResp.StatusCode)
	}
	var rlBody struct {
		Limits map[domain.Platform]ratelimit.Status `json:"limits"`
	}
	if err := json.NewDecoder(rlResp.Body).Decode(&rlBody); err != nil {
		t.Fatalf("decode rate-limits: %v", err)
	}
	if rlBody.Limits[domain.PlatformTikTok].Remaining != 2 || rlBody.Limits[domain.PlatformTikTok].Limit != 2 {
		t.Fatalf("unexpected initial tiktok quota: %+v", rlBody.Limits[domain.PlatformTikTok])
	}

	r1, err := http.Post(srv.URL+"/api/tiktok/fetch", "application/json", bytes.NewBufferString(`{"hashtag":"kuliner"}`))
	if err != nil {
		t.Fatalf("fetch 1 failed: %v", err)
	}
	var body1 struct {
		RateLimit ratelimit.Status `json:"rateLimit"`
	}
	_ = json.NewDecoder(r1.Body).Decode(&body1)
	r1.Body.Close()
	if r1.StatusCode != http.StatusOK || body1.RateLimit.Remaining != 1 {
		t.Fatalf("fetch 1 unexpected: status=%d rl=%+v", r1.StatusCode, body1.RateLimit)
	}

	r2, err := http.Post(srv.URL+"/api/tiktok/fetch", "application/json", bytes.NewBufferString(`{"hashtag":"throttle"}`))
	if err != nil {
		t.Fatalf("fetch 2 failed: %v", err)
	}
	var body2 struct {
		RateLimit ratelimit.Status `json:"rateLimit"`
	}
	_ = json.NewDecoder(r2.Body).Decode(&body2)
	r2.Body.Close()
	if r2.StatusCode != http.StatusTooManyRequests || !body2.RateLimit.UpstreamBlocked || body2.RateLimit.RetryAfterSeconds != 120 {
		t.Fatalf("fetch 2 unexpected: status=%d rl=%+v", r2.StatusCode, body2.RateLimit)
	}

	r3, err := http.Post(srv.URL+"/api/tiktok/fetch", "application/json", bytes.NewBufferString(`{"hashtag":"kuliner"}`))
	if err != nil {
		t.Fatalf("fetch 3 failed: %v", err)
	}
	r3.Body.Close()
	if r3.StatusCode != http.StatusTooManyRequests || r3.Header.Get("Retry-After") != "120" {
		t.Fatalf("fetch 3 expected 429 Retry-After=120, got status=%d header=%q", r3.StatusCode, r3.Header.Get("Retry-After"))
	}
}

func TestAPITikTokUnlimitedCacheAndForceRefresh(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "tiktok-cache.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 11, 23, 8, 0, 0, 0, time.UTC)
	var upstreamCalls int
	upstreamTikTok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("challengeName")
		if tag == "throttle" {
			w.Header().Set("Retry-After", "90")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"statusCode":0,"extra":{"now":` + strconv.FormatInt(now.UnixMilli(), 10) + `},"challengeInfo":{"challenge":{"title":"` + tag + `"},"statsV2":{"videoCount":"100","viewCount":"` + strconv.FormatInt(int64(5000+upstreamCalls), 10) + `"}}}`))
	}))
	defer upstreamTikTok.Close()

	lim := ratelimit.NewLimiter(nil).WithClock(func() time.Time { return now })
	repo := store.NewRepository(db)
	runner := service.NewRunner(repo, apify.NewClient(""), config.Config{})
	defer func() { _ = runner.Shutdown(context.Background()) }()

	handler := NewHandler(repo, runner).
		WithTikTokClient(tiktok.NewClient(upstreamTikTok.URL)).
		WithRateLimiter(lim).
		WithTikTokMinInterval(0).
		WithTikTokCacheTTL(5 * time.Minute).
		WithClock(func() time.Time { return now })
	srv := httptest.NewServer(NewRouter(handler, RouterOptions{}))
	defer srv.Close()

	type fetchResp struct {
		Hashtag   string                `json:"hashtag"`
		Summary   domain.HashtagSummary `json:"summary"`
		RateLimit ratelimit.Status      `json:"rateLimit"`
		Cached    bool                  `json:"cached"`
	}

	postFetch := func(payload string) (int, fetchResp) {
		resp, err := http.Post(srv.URL+"/api/tiktok/fetch", "application/json", bytes.NewBufferString(payload))
		if err != nil {
			t.Fatalf("POST /api/tiktok/fetch(%s) failed: %v", payload, err)
		}
		defer resp.Body.Close()
		var out fetchResp
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	code, f1 := postFetch(`{"hashtag":"kuliner"}`)
	if code != http.StatusOK || f1.Cached || !f1.RateLimit.Unlimited || upstreamCalls != 1 {
		t.Fatalf("step 1 unexpected: code=%d cached=%v unlimited=%v calls=%d", code, f1.Cached, f1.RateLimit.Unlimited, upstreamCalls)
	}

	now = now.Add(1 * time.Minute)
	code, f2 := postFetch(`{"hashtag":"kuliner"}`)
	if code != http.StatusOK || !f2.Cached || upstreamCalls != 1 || len(f2.Summary.RunHistory) != 1 {
		t.Fatalf("step 2 cache hit failed: code=%d cached=%v calls=%d history=%d", code, f2.Cached, upstreamCalls, len(f2.Summary.RunHistory))
	}

	code, f3 := postFetch(`{"hashtag":"kuliner","force":true}`)
	if code != http.StatusOK || f3.Cached || upstreamCalls != 2 || len(f3.Summary.RunHistory) != 2 {
		t.Fatalf("step 3 force refresh failed: code=%d cached=%v calls=%d history=%d", code, f3.Cached, upstreamCalls, len(f3.Summary.RunHistory))
	}

	now = now.Add(6 * time.Minute)
	code, f4 := postFetch(`{"hashtag":"kuliner"}`)
	if code != http.StatusOK || f4.Cached || upstreamCalls != 3 {
		t.Fatalf("step 4 expired cache failed: code=%d cached=%v calls=%d", code, f4.Cached, upstreamCalls)
	}

	for i := 1; i <= 15; i++ {
		code, fi := postFetch(`{"hashtag":"tag` + strconv.Itoa(i) + `"}`)
		if code != http.StatusOK || fi.Cached {
			t.Fatalf("step 5 tag%d unexpected: code=%d cached=%v", i, code, fi.Cached)
		}
	}

	code, f6 := postFetch(`{"hashtag":"throttle"}`)
	if code != http.StatusTooManyRequests || !f6.RateLimit.UpstreamBlocked || f6.RateLimit.RetryAfterSeconds != 90 {
		t.Fatalf("step 6 upstream 429 failed: code=%d rl=%+v", code, f6.RateLimit)
	}

	code, _ = postFetch(`{"hashtag":"afterthrottle"}`)
	if code != http.StatusTooManyRequests {
		t.Fatalf("step 6 uncached during cooldown expected 429, got %d", code)
	}

	now = now.Add(91 * time.Second)
	code, _ = postFetch(`{"hashtag":"afterthrottle"}`)
	if code != http.StatusOK {
		t.Fatalf("step 6 after cooldown expected 200, got %d", code)
	}
}

func TestAPIFeatureFlagsDisablePlatformsAndMockMode(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "flags-api.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	upstreamTikTok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"statusCode":0,"extra":{"now":1795164626000},"challengeInfo":{"challenge":{"title":"kuliner"},"statsV2":{"videoCount":"120","viewCount":"850000"}}}`))
	}))
	defer upstreamTikTok.Close()

	repo := store.NewRepository(db)
	runner := service.NewRunner(repo, apify.NewClient(""), config.Config{})
	defer func() { _ = runner.Shutdown(context.Background()) }()

	ttOnlyHandler := NewHandler(repo, runner).
		WithTikTokClient(tiktok.NewClient(upstreamTikTok.URL)).
		WithTikTokMinInterval(0).
		WithEnabledPlatforms([]string{"tiktok"}).
		WithAllowMockMode(false)
	ttSrv := httptest.NewServer(NewRouter(ttOnlyHandler, RouterOptions{}))
	defer ttSrv.Close()

	healthResp, err := http.Get(ttSrv.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer healthResp.Body.Close()
	var healthBody struct {
		EnabledPlatforms []domain.Platform                  `json:"enabledPlatforms"`
		AllowMockMode    bool                               `json:"allowMockMode"`
		RateLimits       map[domain.Platform]ratelimit.Status `json:"rateLimits"`
	}
	if err := json.NewDecoder(healthResp.Body).Decode(&healthBody); err != nil {
		t.Fatalf("decode /api/health: %v", err)
	}
	if len(healthBody.EnabledPlatforms) != 1 || healthBody.EnabledPlatforms[0] != domain.PlatformTikTok || healthBody.AllowMockMode {
		t.Fatalf("unexpected health flags: %+v", healthBody)
	}
	if _, ok := healthBody.RateLimits[domain.PlatformInstagram]; ok {
		t.Fatalf("expected instagram omitted from health rateLimits: %+v", healthBody.RateLimits)
	}
	if _, ok := healthBody.RateLimits[domain.PlatformTikTok]; !ok {
		t.Fatalf("expected tiktok present in health rateLimits: %+v", healthBody.RateLimits)
	}

	assertStatus := func(method, urlStr, body string, want int) {
		t.Helper()
		var reader io.Reader
		if body != "" {
			reader = bytes.NewBufferString(body)
		}
		req, err := http.NewRequest(method, urlStr, reader)
		if err != nil {
			t.Fatalf("NewRequest %s %s: %v", method, urlStr, err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s failed: %v", method, urlStr, err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s %s status = %d, want %d", method, urlStr, resp.StatusCode, want)
		}
	}

	assertStatus(http.MethodGet, ttSrv.URL+"/api/hashtags?platform=instagram", "", http.StatusForbidden)
	assertStatus(http.MethodGet, ttSrv.URL+"/api/hashtags/gta/summary?platform=facebook", "", http.StatusForbidden)
	assertStatus(http.MethodGet, ttSrv.URL+"/api/hashtags/gta/videos?platform=instagram", "", http.StatusForbidden)
	assertStatus(http.MethodGet, ttSrv.URL+"/api/hashtags/gta/export?platform=instagram", "", http.StatusForbidden)
	assertStatus(http.MethodPost, ttSrv.URL+"/api/runs", `{"hashtag":"gta","platforms":["instagram"],"limit":10,"mode":"mock"}`, http.StatusForbidden)
	assertStatus(http.MethodPost, ttSrv.URL+"/api/runs", `{"hashtag":"gta","platforms":["instagram"],"limit":10,"mode":"live"}`, http.StatusForbidden)
	assertStatus(http.MethodPost, ttSrv.URL+"/api/runs", `{"hashtag":"gta","limit":10,"mode":"live"}`, http.StatusForbidden)
	assertStatus(http.MethodPost, ttSrv.URL+"/api/tiktok/fetch", `{"hashtag":"kuliner"}`, http.StatusOK)

	igOnlyHandler := NewHandler(repo, runner).
		WithEnabledPlatforms([]string{"instagram"}).
		WithAllowMockMode(false)
	igSrv := httptest.NewServer(NewRouter(igOnlyHandler, RouterOptions{}))
	defer igSrv.Close()

	assertStatus(http.MethodPost, igSrv.URL+"/api/runs", `{"hashtag":"gta","platforms":["instagram"],"limit":10,"mode":"mock"}`, http.StatusForbidden)
	assertStatus(http.MethodPost, igSrv.URL+"/api/tiktok/fetch", `{"hashtag":"kuliner"}`, http.StatusForbidden)
}
