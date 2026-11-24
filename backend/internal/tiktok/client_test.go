package tiktok

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"meta-hashtag-views/internal/parser"
)

func TestClientFetchChallengeDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/challenge/detail/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		tag := r.URL.Query().Get("challengeName")
		w.Header().Set("Content-Type", "application/json")
		switch tag {
		case "kuliner":
			_, _ = w.Write([]byte(`{"challengeInfo":{"challenge":{"title":"kuliner"},"statsV2":{"videoCount":"4721643","viewCount":"84098188263"}},"extra":{"now":1795164626000},"statusCode":0}`))
		case "tidakada":
			_, _ = w.Write([]byte(`{"statusCode":10205}`))
		case "ratelimit":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	ctx := context.Background()

	got, err := client.FetchChallengeDetail(ctx, "#Kuliner")
	if err != nil {
		t.Fatalf("FetchChallengeDetail(#Kuliner) error: %v", err)
	}
	if got.Hashtag != "kuliner" || got.TotalVideos != 4721643 || got.TotalViews != 84098188263 {
		t.Fatalf("unexpected stats: %+v", got)
	}

	_, err = client.FetchChallengeDetail(ctx, "tidakada")
	if !errors.Is(err, parser.ErrTikTokHashtagNotFound) {
		t.Fatalf("expected ErrTikTokHashtagNotFound, got: %v", err)
	}

	_, err = client.FetchChallengeDetail(ctx, "ratelimit")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got: %v", err)
	}
}
