package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"meta-hashtag-views/internal/domain"
)

func int64Ptr(v int64) *int64 { return &v }

func TestRepositoryDeduplicationAndDelta(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	run1, err := repo.CreateRun(ctx, domain.RunOptions{
		Hashtag:   "gta",
		Platforms: []domain.Platform{domain.PlatformInstagram, domain.PlatformFacebook},
		Limit:     10,
		Mode:      domain.ModeMock,
	})
	if err != nil {
		t.Fatalf("CreateRun 1 failed: %v", err)
	}

	batch1 := []domain.NormalizedVideo{
		{
			Platform:    domain.PlatformInstagram,
			VideoID:     "ig_1",
			Hashtag:     "gta",
			Permalink:   "https://www.instagram.com/reel/ig_1/",
			AuthorName:  "gamer1",
			Caption:     "First GTA clip #gta",
			MediaType:   "reel",
			PublishedAt: "2026-11-18T10:00:00Z",
			ViewsCount:  int64Ptr(1000),
			ViewsSource: "videoPlayCount",
			LikesCount:  int64Ptr(50),
		},
		{
			Platform:    domain.PlatformFacebook,
			VideoID:     "fb_1",
			Hashtag:     "gta",
			Permalink:   "https://www.facebook.com/reel/fb_1/",
			AuthorName:  "FB Gamer",
			Caption:     "FB reel #gta",
			MediaType:   "reel",
			PublishedAt: "2026-11-18T11:00:00Z",
			ViewsCount:  int64Ptr(500),
			ViewsSource: "playCount",
		},
		{
			Platform:    domain.PlatformFacebook,
			VideoID:     "fb_2",
			Hashtag:     "gta",
			Permalink:   "https://www.facebook.com/watch/?v=fb_2",
			AuthorName:  "FB Feed Page",
			Caption:     "Feed video without public views #gta",
			MediaType:   "video",
			PublishedAt: "2026-11-18T12:00:00Z",
			ViewsCount:  nil,
			ViewsSource: "unavailable",
		},
	}

	if err := repo.SaveVideosAndSnapshots(ctx, run1.ID, "gta", batch1); err != nil {
		t.Fatalf("SaveVideosAndSnapshots 1 failed: %v", err)
	}
	if err := repo.FinalizeRun(ctx, run1.ID, RunCompletionUpdate{
		Status:             domain.StatusCompleted,
		RawItemsCount:      4,
		VideoItemsCount:    3,
		MissingViewsCount:  1,
		TotalViewsSnapshot: 1500,
	}); err != nil {
		t.Fatalf("FinalizeRun 1 failed: %v", err)
	}

	sum1, err := repo.GetHashtagSummary(ctx, "gta")
	if err != nil {
		t.Fatalf("GetHashtagSummary 1 failed: %v", err)
	}
	if sum1.GrandTotalViews != 1500 || sum1.TotalVideos != 3 || sum1.VideosMissingViews != 1 {
		t.Fatalf("unexpected summary 1: %+v", sum1)
	}
	if sum1.DeltaViews != nil {
		t.Fatalf("expected nil delta on first run, got %d", *sum1.DeltaViews)
	}

	run2, err := repo.CreateRun(ctx, domain.RunOptions{
		Hashtag:   "gta",
		Platforms: []domain.Platform{domain.PlatformInstagram, domain.PlatformFacebook},
		Limit:     10,
		Mode:      domain.ModeMock,
	})
	if err != nil {
		t.Fatalf("CreateRun 2 failed: %v", err)
	}

	batch2 := []domain.NormalizedVideo{
		{
			Platform:    domain.PlatformInstagram,
			VideoID:     "ig_1",
			Hashtag:     "gta",
			Permalink:   "https://www.instagram.com/reel/ig_1/",
			AuthorName:  "gamer1",
			Caption:     "First GTA clip #gta",
			MediaType:   "reel",
			PublishedAt: "2026-11-18T10:00:00Z",
			ViewsCount:  int64Ptr(1400),
			ViewsSource: "videoPlayCount",
		},
		{
			Platform:    domain.PlatformInstagram,
			VideoID:     "ig_2",
			Hashtag:     "gta",
			Permalink:   "https://www.instagram.com/reel/ig_2/",
			AuthorName:  "gamer2",
			Caption:     "Second GTA clip #gta",
			MediaType:   "reel",
			PublishedAt: "2026-11-19T09:00:00Z",
			ViewsCount:  int64Ptr(600),
			ViewsSource: "videoPlayCount",
		},
		{
			Platform:    domain.PlatformFacebook,
			VideoID:     "fb_1",
			Hashtag:     "gta",
			Permalink:   "https://www.facebook.com/reel/fb_1/",
			AuthorName:  "FB Gamer",
			Caption:     "FB reel #gta",
			MediaType:   "reel",
			PublishedAt: "2026-11-18T11:00:00Z",
			ViewsCount:  int64Ptr(500),
			ViewsSource: "playCount",
		},
	}

	if err := repo.SaveVideosAndSnapshots(ctx, run2.ID, "gta", batch2); err != nil {
		t.Fatalf("SaveVideosAndSnapshots 2 failed: %v", err)
	}
	if err := repo.FinalizeRun(ctx, run2.ID, RunCompletionUpdate{
		Status:             domain.StatusCompleted,
		RawItemsCount:      3,
		VideoItemsCount:    3,
		MissingViewsCount:  0,
		TotalViewsSnapshot: 2500,
	}); err != nil {
		t.Fatalf("FinalizeRun 2 failed: %v", err)
	}

	sum2, err := repo.GetHashtagSummary(ctx, "gta")
	if err != nil {
		t.Fatalf("GetHashtagSummary 2 failed: %v", err)
	}
	if sum2.GrandTotalViews != 2500 {
		t.Fatalf("got GrandTotalViews=%d, want 2500 (deduplicated)", sum2.GrandTotalViews)
	}
	if sum2.TotalVideos != 4 {
		t.Fatalf("got TotalVideos=%d, want 4", sum2.TotalVideos)
	}
	if sum2.DeltaViews == nil || *sum2.DeltaViews != 1000 {
		t.Fatalf("got DeltaViews=%v, want 1000", sum2.DeltaViews)
	}
	if sum2.DeltaVideos == nil || *sum2.DeltaVideos != 1 {
		t.Fatalf("got DeltaVideos=%v, want 1", sum2.DeltaVideos)
	}

	igStats := sum2.Platforms[domain.PlatformInstagram]
	if igStats.TotalViews != 2000 || igStats.TotalVideos != 2 {
		t.Fatalf("unexpected IG breakdown: %+v", igStats)
	}

	fbVideos, err := repo.ListHashtagVideos(ctx, "gta", domain.VideoQueryOptions{
		Platform: "facebook",
		Sort:     "views_desc",
	})
	if err != nil {
		t.Fatalf("ListHashtagVideos failed: %v", err)
	}
	if len(fbVideos) != 2 || fbVideos[0].VideoID != "fb_1" || fbVideos[1].ViewsCount != nil {
		t.Fatalf("unexpected FB videos ordering: %+v", fbVideos)
	}

	tracked, err := repo.ListTrackedHashtags(ctx)
	if err != nil {
		t.Fatalf("ListTrackedHashtags failed: %v", err)
	}
	if len(tracked) != 1 || tracked[0].GrandTotalViews != 2500 || tracked[0].TotalVideos != 4 {
		t.Fatalf("unexpected tracked hashtags: %+v", tracked)
	}

	igOnly, err := repo.GetHashtagSummaryByPlatform(ctx, "gta", domain.PlatformInstagram)
	if err != nil {
		t.Fatalf("GetHashtagSummaryByPlatform(ig) failed: %v", err)
	}
	if igOnly.GrandTotalViews != 2000 || igOnly.TotalVideos != 2 || igOnly.VideosMissingViews != 0 {
		t.Fatalf("unexpected IG-only summary: %+v", igOnly)
	}
	if igOnly.DeltaViews == nil || *igOnly.DeltaViews != 1000 || igOnly.DeltaVideos == nil || *igOnly.DeltaVideos != 1 {
		t.Fatalf("unexpected IG-only delta: views=%v videos=%v", igOnly.DeltaViews, igOnly.DeltaVideos)
	}

	fbOnly, err := repo.GetHashtagSummaryByPlatform(ctx, "gta", domain.PlatformFacebook)
	if err != nil {
		t.Fatalf("GetHashtagSummaryByPlatform(fb) failed: %v", err)
	}
	if fbOnly.GrandTotalViews != 500 || fbOnly.TotalVideos != 2 || fbOnly.VideosMissingViews != 1 {
		t.Fatalf("unexpected FB-only summary: %+v", fbOnly)
	}
}

func TestOpenSQLiteMigrationsAreIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migrate.db")

	db1, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("first OpenSQLite failed: %v", err)
	}
	_ = db1.Close()

	db2, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("second OpenSQLite failed: %v", err)
	}
	defer db2.Close()

	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = '001_init.sql'`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 applied 001_init.sql record, got %d", count)
	}
	if err := db2.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = '002_aggregate_snapshots.sql'`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations 002 failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 applied 002_aggregate_snapshots.sql record, got %d", count)
	}
}

func TestAggregateSnapshotsForTikTok(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tiktok.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	t1 := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)

	snap1, err := repo.SaveAggregateSnapshot(ctx, domain.PlatformTikTok, "gta", 12450000, 150234567890, "tiktok_challenge_detail", t1)
	if err != nil {
		t.Fatalf("SaveAggregateSnapshot 1 failed: %v", err)
	}
	if snap1.ID <= 0 || snap1.TotalViews != 150234567890 {
		t.Fatalf("unexpected snap1: %+v", snap1)
	}

	sum1, err := repo.GetHashtagSummaryByPlatform(ctx, "gta", domain.PlatformTikTok)
	if err != nil {
		t.Fatalf("GetHashtagSummaryByPlatform 1 failed: %v", err)
	}
	if !sum1.IsAggregateOnly || sum1.GrandTotalViews != 150234567890 || sum1.TotalVideos != 12450000 {
		t.Fatalf("unexpected TikTok summary 1: %+v", sum1)
	}
	if sum1.DeltaViews != nil || sum1.DeltaVideos != nil {
		t.Fatalf("expected nil delta on first snapshot, got views=%v videos=%v", sum1.DeltaViews, sum1.DeltaVideos)
	}

	if _, err := repo.SaveAggregateSnapshot(ctx, domain.PlatformTikTok, "gta", 12450500, 150250000000, "tiktok_challenge_detail", t2); err != nil {
		t.Fatalf("SaveAggregateSnapshot 2 failed: %v", err)
	}

	sum2, err := repo.GetHashtagSummaryByPlatform(ctx, "gta", domain.PlatformTikTok)
	if err != nil {
		t.Fatalf("GetHashtagSummaryByPlatform 2 failed: %v", err)
	}
	if !sum2.IsAggregateOnly || sum2.GrandTotalViews != 150250000000 || sum2.TotalVideos != 12450500 {
		t.Fatalf("unexpected TikTok summary 2: %+v", sum2)
	}
	if sum2.DeltaViews == nil || *sum2.DeltaViews != 15432110 {
		t.Fatalf("got DeltaViews=%v, want 15432110", sum2.DeltaViews)
	}
	if sum2.DeltaVideos == nil || *sum2.DeltaVideos != 500 {
		t.Fatalf("got DeltaVideos=%v, want 500", sum2.DeltaVideos)
	}
	if len(sum2.RunHistory) != 2 || sum2.RunHistory[0].Mode != domain.ModeDirect {
		t.Fatalf("unexpected TikTok run history: %+v", sum2.RunHistory)
	}

	tracked, err := repo.ListTrackedHashtagsByPlatform(ctx, domain.PlatformTikTok)
	if err != nil {
		t.Fatalf("ListTrackedHashtagsByPlatform(tiktok) failed: %v", err)
	}
	if len(tracked) != 1 || tracked[0].Hashtag != "gta" || tracked[0].GrandTotalViews != 150250000000 || tracked[0].TikTokViews != 150250000000 {
		t.Fatalf("unexpected tracked TikTok hashtags: %+v", tracked)
	}
}
