package apify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL        = "https://api.apify.com/v2"
	defaultWaitForFinish  = 60
	defaultPageLimit      = 1000
	defaultPollInterval   = 2 * time.Second
	defaultRetryBaseDelay = 500 * time.Millisecond
	maxRateLimitRetries   = 3
	maxErrorBodyBytes     = 64 * 1024
)

type Client struct {
	token          string
	baseURL        string
	httpClient     *http.Client
	pollInterval   time.Duration
	retryBaseDelay time.Duration
	pageLimit      int
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

func WithPollInterval(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.pollInterval = d
		}
	}
}

func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		token:   strings.TrimSpace(token),
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		pollInterval:   defaultPollInterval,
		retryBaseDelay: defaultRetryBaseDelay,
		pageLimit:      defaultPageLimit,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) HasToken() bool {
	return c != nil && c.token != ""
}

type actorRunEnvelope struct {
	Data actorRunData `json:"data"`
}

type actorRunData struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	DefaultDatasetID string `json:"defaultDatasetId"`
}

func (c *Client) RunActorAndFetchDataset(ctx context.Context, actorID string, input any) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, errors.New("APIFY_API_TOKEN belum dikonfigurasi di .env (gunakan mode --mock untuk demo tanpa token)")
	}

	slug := normalizeActorSlug(actorID)
	if slug == "" {
		return nil, errors.New("actor ID Apify tidak boleh kosong")
	}

	runData, err := c.startActorRun(ctx, slug, input)
	if err != nil {
		return nil, err
	}

	for runData.Status == "READY" || runData.Status == "RUNNING" {
		if err := sleepWithContext(ctx, c.pollInterval); err != nil {
			return nil, fmt.Errorf("menunggu actor %s dibatalkan: %w", actorID, err)
		}
		runData, err = c.getActorRun(ctx, runData.ID)
		if err != nil {
			return nil, err
		}
	}

	if runData.Status != "SUCCEEDED" {
		return nil, fmt.Errorf("actor Apify %s berakhir dengan status %s (runId=%s)", actorID, runData.Status, runData.ID)
	}
	if runData.DefaultDatasetID == "" {
		return nil, fmt.Errorf("actor Apify %s tidak mengembalikan defaultDatasetId (runId=%s)", actorID, runData.ID)
	}

	return c.fetchDatasetItems(ctx, runData.DefaultDatasetID)
}

func (c *Client) startActorRun(ctx context.Context, slug string, input any) (actorRunData, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return actorRunData{}, fmt.Errorf("marshal input actor: %w", err)
	}

	endpoint := fmt.Sprintf("%s/acts/%s/runs?waitForFinish=%d", c.baseURL, url.PathEscape(slug), defaultWaitForFinish)
	var env actorRunEnvelope
	if err := c.doJSON(ctx, http.MethodPost, endpoint, payload, &env); err != nil {
		return actorRunData{}, fmt.Errorf("start actor %s: %w", slug, err)
	}
	return env.Data, nil
}

func (c *Client) getActorRun(ctx context.Context, runID string) (actorRunData, error) {
	endpoint := fmt.Sprintf("%s/actor-runs/%s?waitForFinish=%d", c.baseURL, url.PathEscape(runID), defaultWaitForFinish)
	var env actorRunEnvelope
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &env); err != nil {
		return actorRunData{}, fmt.Errorf("poll run %s: %w", runID, err)
	}
	return env.Data, nil
}

func (c *Client) fetchDatasetItems(ctx context.Context, datasetID string) ([]map[string]any, error) {
	var all []map[string]any
	offset := 0
	for {
		endpoint := fmt.Sprintf(
			"%s/datasets/%s/items?format=json&clean=true&limit=%d&offset=%d",
			c.baseURL,
			url.PathEscape(datasetID),
			c.pageLimit,
			offset,
		)
		var page []map[string]any
		if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &page); err != nil {
			return nil, fmt.Errorf("fetch dataset %s (offset=%d): %w", datasetID, offset, err)
		}
		all = append(all, page...)
		if len(page) < c.pageLimit {
			break
		}
		offset += len(page)
	}
	if all == nil {
		all = []map[string]any{}
	}
	return all, nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, payload []byte, out any) error {
	for attempt := 0; attempt <= maxRateLimitRetries; attempt++ {
		var bodyReader io.Reader
		if payload != nil {
			bodyReader = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRateLimitRetries {
			delay := parseRetryAfter(resp.Header.Get("Retry-After"), c.retryBaseDelay<<attempt)
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
			resp.Body.Close()
			if err := sleepWithContext(ctx, delay); err != nil {
				return err
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errSnippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
			resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errSnippet)))
		}

		dec := json.NewDecoder(resp.Body)
		dec.UseNumber()
		decodeErr := dec.Decode(out)
		resp.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode response JSON: %w", decodeErr)
		}
		return nil
	}
	return errors.New("rate limit Apify terlampaui setelah beberapa kali percobaan")
}

func parseRetryAfter(header string, fallback time.Duration) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs > 0 && secs <= 30 {
		return time.Duration(secs) * time.Second
	}
	return fallback
}

func normalizeActorSlug(actorID string) string {
	trimmed := strings.TrimSpace(actorID)
	return strings.ReplaceAll(trimmed, "/", "~")
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
