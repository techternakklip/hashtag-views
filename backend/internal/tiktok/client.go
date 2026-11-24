package tiktok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"meta-hashtag-views/internal/parser"
)

const (
	defaultBaseURL = "https://www.tiktok.com"
	maxBodyBytes   = 2 << 20
)

var ErrRateLimited = errors.New("TikTok sedang membatasi permintaan (rate limit). Tunggu sebentar lalu coba lagi.")

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return ErrRateLimited.Error() }
func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		baseURL: base,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) FetchChallengeDetail(ctx context.Context, hashtag string) (parser.TikTokChallengeStats, error) {
	tag, err := parser.NormalizeHashtag(hashtag)
	if err != nil {
		return parser.TikTokChallengeStats{}, err
	}

	endpoint := fmt.Sprintf("%s/api/challenge/detail/?challengeName=%s", c.baseURL, url.QueryEscape(tag))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return parser.TikTokChallengeStats{}, fmt.Errorf("buat request TikTok: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return parser.TikTokChallengeStats{}, fmt.Errorf("gagal menghubungi TikTok: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return parser.TikTokChallengeStats{}, &RateLimitError{
			RetryAfter: parseRetryAfterHeader(resp.Header.Get("Retry-After")),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return parser.TikTokChallengeStats{}, fmt.Errorf("TikTok mengembalikan status HTTP %d; coba lagi beberapa saat", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return parser.TikTokChallengeStats{}, fmt.Errorf("gagal membaca respons TikTok: %w", err)
	}

	stats, err := parser.ParseTikTokChallengeDetail(body)
	if err != nil {
		return parser.TikTokChallengeStats{}, err
	}
	if stats.Hashtag == "" {
		stats.Hashtag = tag
	}
	return stats, nil
}

func parseRetryAfterHeader(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(raw); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
