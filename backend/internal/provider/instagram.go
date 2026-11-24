package provider

import (
	"context"
	"fmt"
	"strings"

	"meta-hashtag-views/internal/domain"
)

var igViewsSpecs = []fieldSpec{
	{path: []string{"videoPlayCount"}, source: "videoPlayCount"},
	{path: []string{"playCount"}, source: "playCount"},
	{path: []string{"ig_play_count"}, source: "ig_play_count"},
	{path: []string{"videoViewCount"}, source: "videoViewCount"},
	{path: []string{"viewCount"}, source: "viewCount"},
	{path: []string{"views"}, source: "views"},
}

type InstagramProvider struct {
	runner  ActorRunner
	actorID string
}

func NewInstagramProvider(runner ActorRunner, actorID string) *InstagramProvider {
	return &InstagramProvider{
		runner:  runner,
		actorID: actorID,
	}
}

func (p *InstagramProvider) Platform() domain.Platform {
	return domain.PlatformInstagram
}

func (p *InstagramProvider) FetchVideos(ctx context.Context, opts domain.RunOptions) (domain.ProviderResult, error) {
	resultsType := "reels"
	if strings.EqualFold(strings.TrimSpace(opts.IGResultsType), "posts") {
		resultsType = "posts"
	}

	input := map[string]any{
		"hashtags":      []string{opts.Hashtag},
		"resultsType":   resultsType,
		"resultsLimit":  opts.Limit,
		"keywordSearch": false,
	}

	items, err := p.runner.RunActorAndFetchDataset(ctx, p.actorID, input)
	if err != nil {
		return domain.ProviderResult{Platform: domain.PlatformInstagram}, fmt.Errorf("instagram scraper gagal: %w", err)
	}

	return NormalizeInstagramItems(items, opts.Hashtag), nil
}

func NormalizeInstagramItems(items []map[string]any, hashtag string) domain.ProviderResult {
	return normalizeDatasetItems(domain.PlatformInstagram, items, hashtag, normalizeInstagramItem)
}

func normalizeInstagramItem(raw map[string]any, hashtag string) (domain.NormalizedVideo, bool) {
	rawType := strings.ToLower(firstNonEmptyString(raw, []string{"type"}, []string{"contentType"}, []string{"mediaType"}))
	productType := strings.ToLower(firstNonEmptyString(raw, []string{"productType"}))
	videoURL := firstNonEmptyString(raw, []string{"videoUrl"}, []string{"video_url"})
	viewsCount, viewsSource := pickFirstViews(raw, igViewsSpecs)
	isVideoFlag := asBool(raw["isVideo"]) || asBool(raw["is_video"])

	isVideo := strings.Contains(rawType, "video") ||
		strings.Contains(rawType, "reel") ||
		strings.Contains(productType, "clips") ||
		isVideoFlag ||
		videoURL != "" ||
		viewsCount != nil

	if !isVideo {
		return domain.NormalizedVideo{}, false
	}

	shortCode := firstNonEmptyString(raw, []string{"shortCode"}, []string{"shortcode"}, []string{"code"})
	permalink := firstNonEmptyString(raw, []string{"url"}, []string{"permalink"})
	if permalink == "" && shortCode != "" {
		permalink = "https://www.instagram.com/reel/" + shortCode + "/"
	}

	videoID := shortCode
	if videoID == "" {
		videoID = firstNonEmptyString(raw, []string{"id"}, []string{"pk"})
	}
	if videoID == "" && permalink != "" {
		videoID = fallbackIDFromURL(permalink)
	}
	if videoID == "" {
		return domain.NormalizedVideo{}, false
	}

	mediaType := "video"
	if strings.Contains(rawType, "reel") || strings.Contains(productType, "clips") || strings.Contains(permalink, "/reel/") {
		mediaType = "reel"
	}

	return domain.NormalizedVideo{
		Platform:      domain.PlatformInstagram,
		VideoID:       videoID,
		Hashtag:       hashtag,
		Permalink:     permalink,
		AuthorName:    firstNonEmptyString(raw, []string{"ownerUsername"}, []string{"ownerFullName"}, []string{"owner", "username"}, []string{"author", "username"}),
		Caption:       firstNonEmptyString(raw, []string{"caption"}, []string{"text"}),
		MediaType:     mediaType,
		PublishedAt:   normalizeTimestamp(nestedValue(raw, "timestamp")),
		ViewsCount:    viewsCount,
		ViewsSource:   viewsSource,
		LikesCount:    pickFirstInt(raw, []string{"likesCount"}, []string{"likeCount"}, []string{"like_count"}),
		CommentsCount: pickFirstInt(raw, []string{"commentsCount"}, []string{"commentCount"}, []string{"comment_count"}),
	}, true
}
