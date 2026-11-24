package provider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"meta-hashtag-views/internal/domain"
)

type fakeRunner struct {
	handler func(actorID string, input any) ([]map[string]any, error)
}

func (f *fakeRunner) RunActorAndFetchDataset(_ context.Context, actorID string, input any) ([]map[string]any, error) {
	return f.handler(actorID, input)
}

func TestNormalizeInstagramItems(t *testing.T) {
	items := []map[string]any{
		{
			"id":             "1",
			"shortCode":      "C_reel_1",
			"type":           "Video",
			"productType":    "clips",
			"videoPlayCount": json.Number("125000"),
			"videoViewCount": json.Number("98000"),
			"likesCount":     json.Number("4200"),
			"commentsCount":  json.Number("150"),
			"ownerUsername":  "gta_pro",
			"caption":        "Epic stunt #gta",
			"url":            "https://www.instagram.com/reel/C_reel_1/",
			"timestamp":      "2026-11-18T10:00:00Z",
		},
		{
			"id":        "2",
			"shortCode": "C_img_1",
			"type":      "Image",
			"caption":   "Screenshot #gta",
			"url":       "https://www.instagram.com/p/C_img_1/",
		},
		{
			"id":             "3",
			"shortCode":      "C_reel_1",
			"type":           "Video",
			"videoPlayCount": 130000,
			"url":            "https://www.instagram.com/reel/C_reel_1/",
		},
	}

	res := NormalizeInstagramItems(items, "gta")
	if res.RawItemsCount != 3 || res.SkippedNonVideo != 1 {
		t.Fatalf("unexpected counts: raw=%d skipped=%d", res.RawItemsCount, res.SkippedNonVideo)
	}
	if len(res.Videos) != 1 {
		t.Fatalf("expected 1 deduplicated video, got %d", len(res.Videos))
	}
	v := res.Videos[0]
	if v.ViewsCount == nil || *v.ViewsCount != 130000 || v.ViewsSource != "videoPlayCount" {
		t.Fatalf("unexpected views: %+v", v)
	}
	if v.MediaType != "reel" || v.AuthorName != "gta_pro" {
		t.Fatalf("unexpected metadata: %+v", v)
	}
}

func TestFacebookProviderWithOptionalEnrich(t *testing.T) {
	runner := &fakeRunner{
		handler: func(actorID string, input any) ([]map[string]any, error) {
			if actorID == "apify/facebook-hashtag-scraper" {
				return []map[string]any{
					{
						"postId":     "fb_post_text",
						"postFormat": "text",
						"postText":   "Diskusi #gta",
						"postUrl":    "https://www.facebook.com/user/posts/1",
					},
					{
						"postId":     "fb_vid_direct",
						"postFormat": "video",
						"postUrl":    "https://www.facebook.com/reel/111111/",
						"engagement": map[string]any{"plays": "45.5K", "reactions": 900},
					},
					{
						"postId":     "fb_vid_missing",
						"postFormat": "video",
						"postUrl":    "https://www.facebook.com/reel/222222/",
						"postText":   "Need enrich #gta",
					},
				}, nil
			}
			if actorID == "apify/facebook-reels-scraper" {
				return []map[string]any{
					{
						"reelId":    "222222",
						"url":       "https://www.facebook.com/reel/222222/",
						"playCount": 88400,
					},
				}, nil
			}
			return nil, errors.New("unexpected actor")
		},
	}

	fb := NewFacebookProvider(runner, "apify/facebook-hashtag-scraper", "apify/facebook-reels-scraper")

	noEnrich, err := fb.FetchVideos(context.Background(), domain.RunOptions{
		Hashtag:  "gta",
		Limit:    20,
		EnrichFB: false,
	})
	if err != nil {
		t.Fatalf("FetchVideos without enrich failed: %v", err)
	}
	if noEnrich.SkippedNonVideo != 1 || len(noEnrich.Videos) != 2 {
		t.Fatalf("unexpected counts without enrich: %+v", noEnrich)
	}
	if noEnrich.Videos[0].ViewsCount == nil || *noEnrich.Videos[0].ViewsCount != 45500 {
		t.Fatalf("expected 45500 views on fb_vid_direct, got %+v", noEnrich.Videos[0].ViewsCount)
	}
	if noEnrich.Videos[1].ViewsCount != nil {
		t.Fatalf("expected nil views on fb_vid_missing before enrich, got %d", *noEnrich.Videos[1].ViewsCount)
	}

	enriched, err := fb.FetchVideos(context.Background(), domain.RunOptions{
		Hashtag:  "gta",
		Limit:    20,
		EnrichFB: true,
	})
	if err != nil {
		t.Fatalf("FetchVideos with enrich failed: %v", err)
	}
	if enriched.Videos[1].ViewsCount == nil || *enriched.Videos[1].ViewsCount != 88400 || enriched.Videos[1].ViewsSource != "fb_reels_enrich" {
		t.Fatalf("expected enriched views 88400, got %+v", enriched.Videos[1])
	}
}
