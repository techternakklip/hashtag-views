package exporter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"meta-hashtag-views/internal/domain"
)

func TestToCSV(t *testing.T) {
	views := int64(15000)
	likes := int64(320)
	videos := []domain.VideoViewRow{
		{
			Platform:      domain.PlatformInstagram,
			Hashtag:       "gta",
			VideoID:       "vid_1",
			ViewsCount:    &views,
			ViewsSource:   "videoPlayCount",
			MediaType:     "reel",
			AuthorName:    "=cmd|' /C calc'!A0",
			PublishedAt:   "2026-11-18T10:00:00Z",
			Permalink:     "https://www.instagram.com/reel/vid_1/",
			LikesCount:    &likes,
			LastFetchedAt: "2026-11-20T10:00:00Z",
			Caption:       "Line 1\nLine 2, with comma",
		},
		{
			Platform:      domain.PlatformFacebook,
			Hashtag:       "gta",
			VideoID:       "vid_2",
			ViewsCount:    nil,
			ViewsSource:   "unavailable",
			MediaType:     "video",
			AuthorName:    "Creator Dua",
			Permalink:     "https://www.facebook.com/watch/?v=vid_2",
			LastFetchedAt: "2026-11-20T10:00:00Z",
			Caption:       "+SUM(1,2)",
		},
	}

	raw, err := ToCSV(videos)
	if err != nil {
		t.Fatalf("ToCSV error: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte(utf8BOM)) {
		t.Fatal("expected UTF-8 BOM prefix")
	}

	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte(utf8BOM))))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv parse error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d rows, want 3", len(records))
	}
	if records[1][3] != "15000" {
		t.Fatalf("got views %q, want 15000", records[1][3])
	}
	if !strings.HasPrefix(records[1][6], "'=") {
		t.Fatalf("expected formula sanitization on author_name, got %q", records[1][6])
	}
	if records[2][3] != "" {
		t.Fatalf("expected empty views cell when ViewsCount is nil, got %q", records[2][3])
	}
	if !strings.HasPrefix(records[2][12], "'+") {
		t.Fatalf("expected formula sanitization on caption, got %q", records[2][12])
	}
}

func TestToJSONReport(t *testing.T) {
	summary := domain.HashtagSummary{
		Hashtag:         "gta",
		GrandTotalViews: 500,
		TotalVideos:     1,
	}
	raw, err := ToJSONReport(summary, nil)
	if err != nil {
		t.Fatalf("ToJSONReport error: %v", err)
	}

	var decoded JSONReport
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if decoded.Summary.Hashtag != "gta" || decoded.Videos == nil {
		t.Fatalf("unexpected decoded report: %+v", decoded)
	}
}
