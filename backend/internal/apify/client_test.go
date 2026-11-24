package apify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientRunActorPollAndPaginateDataset(t *testing.T) {
	var pollCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-secret-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/acts/apify~instagram-hashtag-scraper/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":               "run_123",
					"status":           "RUNNING",
					"defaultDatasetId": "ds_456",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v2/actor-runs/run_123":
			n := pollCount.Add(1)
			status := "RUNNING"
			if n >= 2 {
				status = "SUCCEEDED"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":               "run_123",
					"status":           status,
					"defaultDatasetId": "ds_456",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v2/datasets/ds_456/items":
			offset := r.URL.Query().Get("offset")
			if offset == "0" {
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": "1", "videoPlayCount": 12000},
					{"id": "2", "videoPlayCount": 34000},
				})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "3", "videoPlayCount": 56000},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(
		"test-secret-token",
		WithBaseURL(srv.URL+"/v2"),
		WithPollInterval(5*time.Millisecond),
	)
	client.pageLimit = 2

	items, err := client.RunActorAndFetchDataset(context.Background(), "apify/instagram-hashtag-scraper", map[string]any{
		"hashtags": []string{"gta"},
	})
	if err != nil {
		t.Fatalf("RunActorAndFetchDataset error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if pollCount.Load() < 2 {
		t.Fatalf("expected at least 2 poll calls, got %d", pollCount.Load())
	}
}

func TestClientRequiresTokenAndHandlesFailureStatus(t *testing.T) {
	noToken := NewClient("")
	if _, err := noToken.RunActorAndFetchDataset(context.Background(), "apify/instagram-hashtag-scraper", nil); err == nil {
		t.Fatal("expected error when token missing")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":     "run_fail",
				"status": "FAILED",
			},
		})
	}))
	defer srv.Close()

	client := NewClient("secret-abc", WithBaseURL(srv.URL+"/v2"))
	_, err := client.RunActorAndFetchDataset(context.Background(), "apify/facebook-hashtag-scraper", map[string]any{})
	if err == nil {
		t.Fatal("expected error when actor ends with FAILED")
	}
	if strings.Contains(err.Error(), "secret-abc") {
		t.Fatalf("error leaked token: %v", err)
	}
}

func TestClientRetriesHTTP429RateLimit(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if attempts.Add(1) <= 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":               "run_ok",
					"status":           "SUCCEEDED",
					"defaultDatasetId": "ds_ok",
				},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "74201", "playCount": 99000}})
	}))
	defer srv.Close()

	client := NewClient("token-123", WithBaseURL(srv.URL+"/v2"))
	client.retryBaseDelay = 2 * time.Millisecond

	items, err := client.RunActorAndFetchDataset(context.Background(), "clockworks/tiktok-hashtag-scraper", map[string]any{
		"hashtags": []string{"gta"},
	})
	if err != nil {
		t.Fatalf("expected 429 retry to succeed, got error: %v", err)
	}
	if len(items) != 1 || attempts.Load() != 3 {
		t.Fatalf("expected 1 item after 3 attempts, got items=%d attempts=%d", len(items), attempts.Load())
	}
}
