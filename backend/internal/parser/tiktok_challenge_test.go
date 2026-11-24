package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTikTokChallengeDetail(t *testing.T) {
	t.Run("valid JSON payload with stringified statsV2", func(t *testing.T) {
		raw := []byte(`{"challengeInfo":{"challenge":{"id":"13586062","title":"Kuliner"},"stats":{"videoCount":0,"viewCount":84100000000},"statsV2":{"videoCount":"4721643","viewCount":"84098188263"}},"extra":{"now":1795164626000},"statusCode":0}`)
		got, err := ParseTikTokChallengeDetail(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Hashtag != "kuliner" || got.TotalVideos != 4721643 || got.TotalViews != 84098188263 {
			t.Fatalf("unexpected parsed stats: %+v", got)
		}
		if got.CapturedAt.UnixMilli() != 1795164626000 {
			t.Fatalf("unexpected timestamp: %v", got.CapturedAt)
		}
	})

	t.Run("valid JSON payload with numeric statsV2", func(t *testing.T) {
		raw := []byte(`{"challengeInfo":{"challenge":{"title":"skincare"},"statsV2":{"videoCount":85420,"viewCount":912000450}},"statusCode":0}`)
		got, err := ParseTikTokChallengeDetail(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Hashtag != "skincare" || got.TotalVideos != 85420 || got.TotalViews != 912000450 {
			t.Fatalf("unexpected parsed stats: %+v", got)
		}
	})

	t.Run("statusCode 10205 not found", func(t *testing.T) {
		_, err := ParseTikTokChallengeDetail([]byte(`{"statusCode":10205}`))
		if !errors.Is(err, ErrTikTokHashtagNotFound) {
			t.Fatalf("expected ErrTikTokHashtagNotFound, got: %v", err)
		}
	})

	t.Run("non-zero statusCode", func(t *testing.T) {
		_, err := ParseTikTokChallengeDetail([]byte(`{"statusCode":10202}`))
		if err == nil || !strings.Contains(err.Error(), "kode 10202") {
			t.Fatalf("expected statusCode 10202 error, got: %v", err)
		}
	})

	t.Run("missing statsV2", func(t *testing.T) {
		_, err := ParseTikTokChallengeDetail([]byte(`{"statusCode":0,"challengeInfo":{}}`))
		if err == nil {
			t.Fatalf("expected error when statsV2 is missing")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := ParseTikTokChallengeDetail([]byte("not json"))
		if err == nil {
			t.Fatalf("expected error for invalid JSON")
		}
	})
}
