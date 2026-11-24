package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"meta-hashtag-views/internal/domain"
	"meta-hashtag-views/internal/parser"
)

type fieldSpec struct {
	path   []string
	source string
}

func pickFirstViews(m map[string]any, specs []fieldSpec) (*int64, string) {
	for _, spec := range specs {
		raw := nestedValue(m, spec.path...)
		if raw == nil {
			continue
		}
		if parsed := parser.ParseViews(raw); parsed != nil {
			return parsed, spec.source
		}
	}
	return nil, "unavailable"
}

func pickFirstInt(m map[string]any, paths ...[]string) *int64 {
	for _, p := range paths {
		raw := nestedValue(m, p...)
		if raw == nil {
			continue
		}
		if parsed := parser.ParseViews(raw); parsed != nil {
			return parsed
		}
	}
	return nil
}

func firstNonEmptyString(m map[string]any, paths ...[]string) string {
	for _, p := range paths {
		raw := nestedValue(m, p...)
		if s := asString(raw); s != "" {
			return s
		}
	}
	return ""
}

func nestedValue(m map[string]any, path ...string) any {
	var cur any = m
	for _, key := range path {
		asMap, ok := cur.(map[string]any)
		if !ok || asMap == nil {
			return nil
		}
		val, exists := asMap[key]
		if !exists {
			return nil
		}
		cur = val
	}
	return cur
}

func asString(raw any) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func asBool(raw any) bool {
	if raw == nil {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func normalizeTimestamp(raw any) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
		if t, err := time.Parse("2006-01-02T15:04:05-0700", s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
		if secs, err := strconv.ParseInt(s, 10, 64); err == nil {
			return unixToRFC3339(secs)
		}
		return s
	case json.Number:
		if secs, err := v.Int64(); err == nil {
			return unixToRFC3339(secs)
		}
	case float64:
		return unixToRFC3339(int64(v))
	case int64:
		return unixToRFC3339(v)
	}
	return ""
}

func unixToRFC3339(ts int64) string {
	if ts <= 0 {
		return ""
	}
	if ts >= 1_000_000_000_000 {
		return time.UnixMilli(ts).UTC().Format(time.RFC3339)
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func fallbackIDFromURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err == nil {
		if v := u.Query().Get("v"); v != "" {
			return v
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			seg := strings.TrimSpace(parts[i])
			if seg != "" && seg != "reel" && seg != "reels" && seg != "p" && seg != "watch" && seg != "videos" {
				return seg
			}
		}
	}
	sum := sha256.Sum256([]byte(rawURL))
	return "hash_" + hex.EncodeToString(sum[:8])
}

func normalizeDatasetItems(
	platform domain.Platform,
	items []map[string]any,
	hashtag string,
	normalizeOne func(map[string]any, string) (domain.NormalizedVideo, bool),
) domain.ProviderResult {
	res := domain.ProviderResult{
		Platform:      platform,
		RawItemsCount: len(items),
	}
	var videos []domain.NormalizedVideo
	for _, raw := range items {
		v, ok := normalizeOne(raw, hashtag)
		if !ok {
			res.SkippedNonVideo++
			continue
		}
		videos = append(videos, v)
	}
	res.Videos = dedupeProviderVideos(videos)
	return res
}

func dedupeProviderVideos(videos []domain.NormalizedVideo) []domain.NormalizedVideo {
	byKey := make(map[string]int, len(videos))
	out := make([]domain.NormalizedVideo, 0, len(videos))
	for _, v := range videos {
		key := string(v.Platform) + ":" + v.VideoID
		if idx, exists := byKey[key]; exists {
			out[idx] = mergeVideos(out[idx], v)
			continue
		}
		byKey[key] = len(out)
		out = append(out, v)
	}
	return out
}

func mergeVideos(base, incoming domain.NormalizedVideo) domain.NormalizedVideo {
	if incoming.ViewsCount != nil {
		if base.ViewsCount == nil || *incoming.ViewsCount > *base.ViewsCount {
			base.ViewsCount = incoming.ViewsCount
			base.ViewsSource = incoming.ViewsSource
		}
	}
	if base.Permalink == "" {
		base.Permalink = incoming.Permalink
	}
	if base.AuthorName == "" {
		base.AuthorName = incoming.AuthorName
	}
	if base.Caption == "" {
		base.Caption = incoming.Caption
	}
	if base.PublishedAt == "" {
		base.PublishedAt = incoming.PublishedAt
	}
	if base.MediaType == "" || (base.MediaType == "video" && incoming.MediaType == "reel") {
		base.MediaType = incoming.MediaType
	}
	if base.LikesCount == nil {
		base.LikesCount = incoming.LikesCount
	}
	if base.CommentsCount == nil {
		base.CommentsCount = incoming.CommentsCount
	}
	return base
}
