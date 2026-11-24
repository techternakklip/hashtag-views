package provider

import (
	"context"
	"fmt"
	"strings"

	"meta-hashtag-views/internal/domain"
)

const maxFBEnrichURLs = 100

var fbViewsSpecs = []fieldSpec{
	{path: []string{"playCount"}, source: "playCount"},
	{path: []string{"views"}, source: "views"},
	{path: []string{"viewsCount"}, source: "viewsCount"},
	{path: []string{"videoViewCount"}, source: "videoViewCount"},
	{path: []string{"engagement", "plays"}, source: "engagement.plays"},
	{path: []string{"engagement", "views"}, source: "engagement.views"},
	{path: []string{"playCountLabel"}, source: "playCountLabel"},
	{path: []string{"viewsLabel"}, source: "viewsLabel"},
}

type FacebookProvider struct {
	runner         ActorRunner
	hashtagActorID string
	reelsActorID   string
}

func NewFacebookProvider(runner ActorRunner, hashtagActorID, reelsActorID string) *FacebookProvider {
	return &FacebookProvider{
		runner:         runner,
		hashtagActorID: hashtagActorID,
		reelsActorID:   reelsActorID,
	}
}

func (p *FacebookProvider) Platform() domain.Platform {
	return domain.PlatformFacebook
}

func (p *FacebookProvider) FetchVideos(ctx context.Context, opts domain.RunOptions) (domain.ProviderResult, error) {
	input := map[string]any{
		"hashtags":           []string{opts.Hashtag},
		"maxPostsPerHashtag": opts.Limit,
		"maxItems":           opts.Limit,
		"proxyConfiguration": map[string]any{"useApifyProxy": true},
	}

	items, err := p.runner.RunActorAndFetchDataset(ctx, p.hashtagActorID, input)
	if err != nil {
		return domain.ProviderResult{Platform: domain.PlatformFacebook}, fmt.Errorf("facebook scraper gagal: %w", err)
	}

	res := NormalizeFacebookItems(items, opts.Hashtag)
	if !opts.EnrichFB || p.reelsActorID == "" {
		return res, nil
	}

	p.enrichMissingViews(ctx, &res)
	return res, nil
}

func (p *FacebookProvider) enrichMissingViews(ctx context.Context, res *domain.ProviderResult) {
	var startUrls []map[string]string
	seenURL := make(map[string]bool)
	for _, v := range res.Videos {
		if v.ViewsCount != nil || v.Permalink == "" || seenURL[v.Permalink] {
			continue
		}
		seenURL[v.Permalink] = true
		startUrls = append(startUrls, map[string]string{"url": v.Permalink})
		if len(startUrls) >= maxFBEnrichURLs {
			break
		}
	}
	if len(startUrls) == 0 {
		return
	}

	enrichItems, err := p.runner.RunActorAndFetchDataset(ctx, p.reelsActorID, map[string]any{
		"startUrls":          startUrls,
		"resultsLimit":       len(startUrls),
		"proxyConfiguration": map[string]any{"useApifyProxy": true},
	})
	if err != nil {
		res.Notes = append(res.Notes, fmt.Sprintf("FB Reels enrich gagal (hasil discovery tetap disimpan): %v", err))
		return
	}

	byKey := make(map[string]*int64)
	for _, item := range enrichItems {
		views, _ := pickFirstViews(item, fbViewsSpecs)
		if views == nil {
			continue
		}
		for _, candidate := range []string{
			firstNonEmptyString(item, []string{"reelId"}),
			firstNonEmptyString(item, []string{"postId"}),
			firstNonEmptyString(item, []string{"videoId"}),
			firstNonEmptyString(item, []string{"id"}),
			normalizePermalinkKey(firstNonEmptyString(item, []string{"url"}, []string{"postUrl"}, []string{"permalink"})),
		} {
			if candidate != "" {
				byKey[candidate] = views
			}
		}
	}

	for i := range res.Videos {
		if res.Videos[i].ViewsCount != nil {
			continue
		}
		for _, key := range []string{res.Videos[i].VideoID, normalizePermalinkKey(res.Videos[i].Permalink)} {
			if key == "" {
				continue
			}
			if views, ok := byKey[key]; ok && views != nil {
				res.Videos[i].ViewsCount = views
				res.Videos[i].ViewsSource = "fb_reels_enrich"
				break
			}
		}
	}
}

func NormalizeFacebookItems(items []map[string]any, hashtag string) domain.ProviderResult {
	return normalizeDatasetItems(domain.PlatformFacebook, items, hashtag, normalizeFacebookItem)
}

func normalizeFacebookItem(raw map[string]any, hashtag string) (domain.NormalizedVideo, bool) {
	postFormat := strings.ToLower(firstNonEmptyString(raw, []string{"postFormat"}, []string{"type"}, []string{"mediaType"}))
	permalink := firstNonEmptyString(raw, []string{"postUrl"}, []string{"url"}, []string{"permalink"}, []string{"link"})
	videoURL := firstNonEmptyString(raw, []string{"videoUrl"}, []string{"video_url"})

	mediaItem, mediaHasVideo := findVideoMediaEntry(raw)
	if videoURL == "" && mediaItem != nil {
		videoURL = firstNonEmptyString(mediaItem, []string{"videoUrl"}, []string{"url"})
	}
	if permalink == "" && mediaItem != nil {
		permalink = firstNonEmptyString(mediaItem, []string{"url"})
	}

	viewsCount, viewsSource := pickFirstViews(raw, fbViewsSpecs)
	if viewsCount == nil && mediaItem != nil {
		viewsCount, viewsSource = pickFirstViews(mediaItem, fbViewsSpecs)
	}

	isVideo := strings.Contains(postFormat, "video") ||
		strings.Contains(postFormat, "reel") ||
		asBool(raw["isVideo"]) ||
		mediaHasVideo ||
		videoURL != "" ||
		viewsCount != nil ||
		isFacebookVideoURL(permalink)

	if !isVideo {
		return domain.NormalizedVideo{}, false
	}

	videoID := firstNonEmptyString(raw, []string{"reelId"}, []string{"videoId"}, []string{"postId"}, []string{"id"})
	if videoID == "" && permalink != "" {
		videoID = fallbackIDFromURL(permalink)
	}
	if videoID == "" {
		return domain.NormalizedVideo{}, false
	}

	mediaType := "video"
	if strings.Contains(postFormat, "reel") || strings.Contains(strings.ToLower(permalink), "/reel/") {
		mediaType = "reel"
	}

	publishedRaw := nestedValue(raw, "publishedAt")
	if publishedRaw == nil {
		publishedRaw = nestedValue(raw, "time")
	}
	if publishedRaw == nil {
		publishedRaw = nestedValue(raw, "timestamp")
	}

	return domain.NormalizedVideo{
		Platform:    domain.PlatformFacebook,
		VideoID:     videoID,
		Hashtag:     hashtag,
		Permalink:   permalink,
		AuthorName:  firstNonEmptyString(raw, []string{"ownerName"}, []string{"author", "name"}, []string{"pageName"}, []string{"user", "name"}),
		Caption:     firstNonEmptyString(raw, []string{"postText"}, []string{"caption"}, []string{"text"}, []string{"message"}),
		MediaType:   mediaType,
		PublishedAt: normalizeTimestamp(publishedRaw),
		ViewsCount:  viewsCount,
		ViewsSource: viewsSource,
		LikesCount: pickFirstInt(raw,
			[]string{"reactionsCount"},
			[]string{"engagement", "reactions"},
			[]string{"likes"},
			[]string{"likesCount"},
		),
		CommentsCount: pickFirstInt(raw,
			[]string{"commentsCount"},
			[]string{"engagement", "comments"},
			[]string{"comments"},
		),
	}, true
}

func findVideoMediaEntry(raw map[string]any) (map[string]any, bool) {
	for _, key := range []string{"media", "attachments"} {
		slice, ok := raw[key].([]any)
		if !ok {
			continue
		}
		for _, elem := range slice {
			entry, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			mType := strings.ToLower(firstNonEmptyString(entry, []string{"type"}, []string{"__typename"}))
			vURL := firstNonEmptyString(entry, []string{"videoUrl"})
			entryURL := firstNonEmptyString(entry, []string{"url"})
			if strings.Contains(mType, "video") || strings.Contains(mType, "reel") || vURL != "" || isFacebookVideoURL(entryURL) {
				return entry, true
			}
		}
	}
	return nil, false
}

func isFacebookVideoURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "/reel/") ||
		strings.Contains(lower, "/reels/") ||
		strings.Contains(lower, "/videos/") ||
		strings.Contains(lower, "/watch")
}

func normalizePermalinkKey(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	return strings.TrimRight(strings.Split(trimmed, "#")[0], "/")
}
