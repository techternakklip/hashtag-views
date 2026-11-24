package provider

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"meta-hashtag-views/internal/domain"
)

type MockProvider struct {
	platform domain.Platform
	nextStep func(ctx context.Context, hashtag string) (int, error)
}

func NewMockProvider(platform domain.Platform, stepFn func(ctx context.Context, hashtag string) (int, error)) *MockProvider {
	return &MockProvider{
		platform: platform,
		nextStep: stepFn,
	}
}

func (p *MockProvider) Platform() domain.Platform {
	return p.platform
}

func (p *MockProvider) FetchVideos(ctx context.Context, opts domain.RunOptions) (domain.ProviderResult, error) {
	step := 1
	if p.nextStep != nil {
		if s, err := p.nextStep(ctx, opts.Hashtag); err == nil && s > 0 {
			step = s
		}
	}

	seed := hashTagSeed(opts.Hashtag)
	baseVideoCount := 6
	if step > 1 {
		baseVideoCount += min(step-1, 4) * 2
	}
	if opts.Limit > 0 && baseVideoCount > opts.Limit {
		baseVideoCount = opts.Limit
	}

	growthFactor := 1.0 + float64(step-1)*0.18
	videos := make([]domain.NormalizedVideo, 0, baseVideoCount)
	baseTime := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

	for i := 1; i <= baseVideoCount; i++ {
		baseViews := int64(((int(seed)%50)+15)*1000 + i*27500)
		views := int64(float64(baseViews) * growthFactor)
		likes := views / 24
		comments := views / 180
		published := baseTime.Add(time.Duration(i*7) * time.Hour).Format(time.RFC3339)

		if p.platform == domain.PlatformInstagram {
			code := fmt.Sprintf("IG_%s_%02d", opts.Hashtag, i)
			videos = append(videos, domain.NormalizedVideo{
				Platform:      domain.PlatformInstagram,
				VideoID:       code,
				Hashtag:       opts.Hashtag,
				Permalink:     fmt.Sprintf("https://www.instagram.com/reel/%s/", code),
				AuthorName:    fmt.Sprintf("ig_creator_%02d", i),
				Caption:       fmt.Sprintf("[DEMO] Highlight gameplay #%s clip #%d", opts.Hashtag, i),
				MediaType:     "reel",
				PublishedAt:   published,
				ViewsCount:    &views,
				ViewsSource:   "mock_videoPlayCount",
				LikesCount:    &likes,
				CommentsCount: &comments,
			})
			continue
		}

		fbID := fmt.Sprintf("FB_%s_%02d", opts.Hashtag, i)
		var viewsPtr *int64 = &views
		viewsSource := "mock_playCount"
		mediaType := "reel"
		if i == 3 && !opts.EnrichFB {
			viewsPtr = nil
			viewsSource = "unavailable"
			mediaType = "video"
		} else if i == 3 && opts.EnrichFB {
			viewsSource = "fb_reels_enrich"
		}

		videos = append(videos, domain.NormalizedVideo{
			Platform:      domain.PlatformFacebook,
			VideoID:       fbID,
			Hashtag:       opts.Hashtag,
			Permalink:     fmt.Sprintf("https://www.facebook.com/reel/%s/", fbID),
			AuthorName:    fmt.Sprintf("FB Gaming Page %02d", i),
			Caption:       fmt.Sprintf("[DEMO] Komunitas #%s video episode %d", opts.Hashtag, i),
			MediaType:     mediaType,
			PublishedAt:   published,
			ViewsCount:    viewsPtr,
			ViewsSource:   viewsSource,
			LikesCount:    &likes,
			CommentsCount: &comments,
		})
	}

	skippedNonVideo := 2
	return domain.ProviderResult{
		Platform:        p.platform,
		Videos:          videos,
		RawItemsCount:   len(videos) + skippedNonVideo,
		SkippedNonVideo: skippedNonVideo,
	}, nil
}

func hashTagSeed(tag string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(tag))
	return h.Sum32()
}
