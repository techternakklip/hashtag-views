package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"meta-hashtag-views/internal/domain"
	"meta-hashtag-views/internal/provider"
	"meta-hashtag-views/internal/store"
)

type stubProvider struct {
	platform domain.Platform
	res      domain.ProviderResult
	err      error
}

func (s *stubProvider) Platform() domain.Platform { return s.platform }
func (s *stubProvider) FetchVideos(_ context.Context, _ domain.RunOptions) (domain.ProviderResult, error) {
	return s.res, s.err
}

func TestRunnerPartialFailurePreservesSuccessfulPlatform(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "runner.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	repo := store.NewRepository(db)
	v := int64(25000)
	live := map[domain.Platform]provider.HashtagProvider{
		domain.PlatformInstagram: &stubProvider{
			platform: domain.PlatformInstagram,
			res: domain.ProviderResult{
				Platform:      domain.PlatformInstagram,
				RawItemsCount: 2,
				Videos: []domain.NormalizedVideo{
					{
						Platform:    domain.PlatformInstagram,
						VideoID:     "ig_ok_1",
						Hashtag:     "gta",
						Permalink:   "https://www.instagram.com/reel/ig_ok_1/",
						MediaType:   "reel",
						ViewsCount:  &v,
						ViewsSource: "videoPlayCount",
					},
				},
				SkippedNonVideo: 1,
			},
		},
		domain.PlatformFacebook: &stubProvider{
			platform: domain.PlatformFacebook,
			err:      errors.New("apify timeout"),
		},
	}

	runner := NewRunnerWithProviders(repo, live, live, true)
	defer func() { _ = runner.Shutdown(context.Background()) }()

	rec, summary, err := runner.RunSync(context.Background(), domain.RunOptions{
		Hashtag:   "#GTA",
		Platforms: []domain.Platform{domain.PlatformInstagram, domain.PlatformFacebook},
		Limit:     20,
		Mode:      domain.ModeLive,
	})
	if err != nil {
		t.Fatalf("RunSync partial failure should not return error: %v", err)
	}
	if rec.Status != domain.StatusPartial {
		t.Fatalf("got status %s, want partial", rec.Status)
	}
	if summary.GrandTotalViews != 25000 || summary.TotalVideos != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

type cancelOnFetchProvider struct {
	stubProvider
	cancel context.CancelFunc
}

func (c *cancelOnFetchProvider) FetchVideos(ctx context.Context, opts domain.RunOptions) (domain.ProviderResult, error) {
	res, err := c.stubProvider.FetchVideos(ctx, opts)
	c.cancel()
	return res, err
}

func TestRunnerPersistsVideosWhenFetchContextCanceledAfterFetch(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "persist.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	repo := store.NewRepository(db)
	fetchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	v := int64(42000)
	live := map[domain.Platform]provider.HashtagProvider{
		domain.PlatformInstagram: &cancelOnFetchProvider{
			cancel: cancel,
			stubProvider: stubProvider{
				platform: domain.PlatformInstagram,
				res: domain.ProviderResult{
					Platform:      domain.PlatformInstagram,
					RawItemsCount: 1,
					Videos: []domain.NormalizedVideo{
						{
							Platform:    domain.PlatformInstagram,
							VideoID:     "ig_edge_1",
							Hashtag:     "gta",
							Permalink:   "https://www.instagram.com/reel/ig_edge_1/",
							MediaType:   "reel",
							ViewsCount:  &v,
							ViewsSource: "videoPlayCount",
						},
					},
				},
			},
		},
	}

	runner := NewRunnerWithProviders(repo, live, live, true)
	defer func() { _ = runner.Shutdown(context.Background()) }()

	opts := domain.RunOptions{
		Hashtag:   "gta",
		Platforms: []domain.Platform{domain.PlatformInstagram},
		Limit:     10,
		Mode:      domain.ModeLive,
	}
	created, err := repo.CreateRun(context.Background(), opts)
	if err != nil {
		t.Fatalf("CreateRun error: %v", err)
	}

	finalRec := runner.executeRun(fetchCtx, created.ID, opts)
	if finalRec.Status != domain.StatusCompleted || finalRec.TotalViewsSnapshot != 42000 {
		t.Fatalf("expected completed run preserving fetched views, got %+v", finalRec)
	}

	summary, err := repo.GetHashtagSummaryByPlatform(context.Background(), "gta", domain.PlatformInstagram)
	if err != nil {
		t.Fatalf("GetHashtagSummaryByPlatform error: %v", err)
	}
	if summary.GrandTotalViews != 42000 || summary.TotalVideos != 1 {
		t.Fatalf("expected saved video views in SQLite despite canceled fetchCtx, got %+v", summary)
	}
}

func TestRunnerRejectsLiveWithoutToken(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "notoken.db"))
	if err != nil {
		t.Fatalf("OpenSQLite error: %v", err)
	}
	defer db.Close()

	repo := store.NewRepository(db)
	runner := NewRunnerWithProviders(repo, nil, nil, false)
	defer func() { _ = runner.Shutdown(context.Background()) }()

	_, _, err = runner.RunSync(context.Background(), domain.RunOptions{
		Hashtag: "gta",
		Mode:    domain.ModeLive,
	})
	if err == nil {
		t.Fatal("expected error when running live mode without token")
	}
}
