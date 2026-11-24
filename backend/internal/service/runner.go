package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"meta-hashtag-views/internal/apify"
	"meta-hashtag-views/internal/config"
	"meta-hashtag-views/internal/domain"
	"meta-hashtag-views/internal/parser"
	"meta-hashtag-views/internal/provider"
	"meta-hashtag-views/internal/store"
)

const (
	defaultLimit          = 50
	maxAllowedLimit       = 2000
	defaultRunTimeout     = 5 * time.Minute
	defaultPersistTimeout = 10 * time.Second
)

type Runner struct {
	repo          *store.Repository
	liveProviders map[domain.Platform]provider.HashtagProvider
	mockProviders map[domain.Platform]provider.HashtagProvider
	hasApifyToken bool
	baseCtx       context.Context
	cancelBase    context.CancelFunc
	wg            sync.WaitGroup
}

func NewRunner(repo *store.Repository, apifyClient *apify.Client, cfg config.Config) *Runner {
	stepFnFor := func(p domain.Platform) func(ctx context.Context, hashtag string) (int, error) {
		return func(ctx context.Context, hashtag string) (int, error) {
			count, err := repo.CountSuccessfulRunsForPlatform(ctx, hashtag, p)
			if err != nil {
				return 1, err
			}
			return count + 1, nil
		}
	}

	live := map[domain.Platform]provider.HashtagProvider{
		domain.PlatformInstagram: provider.NewInstagramProvider(apifyClient, cfg.IGHashtagActor),
		domain.PlatformFacebook:  provider.NewFacebookProvider(apifyClient, cfg.FBHashtagActor, cfg.FBReelsActor),
	}
	mock := map[domain.Platform]provider.HashtagProvider{
		domain.PlatformInstagram: provider.NewMockProvider(domain.PlatformInstagram, stepFnFor(domain.PlatformInstagram)),
		domain.PlatformFacebook:  provider.NewMockProvider(domain.PlatformFacebook, stepFnFor(domain.PlatformFacebook)),
	}

	return NewRunnerWithProviders(repo, live, mock, apifyClient.HasToken())
}

func NewRunnerWithProviders(
	repo *store.Repository,
	live map[domain.Platform]provider.HashtagProvider,
	mock map[domain.Platform]provider.HashtagProvider,
	hasApifyToken bool,
) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		repo:          repo,
		liveProviders: live,
		mockProviders: mock,
		hasApifyToken: hasApifyToken,
		baseCtx:       ctx,
		cancelBase:    cancel,
	}
}

func (r *Runner) HasApifyToken() bool {
	return r.hasApifyToken
}

func (r *Runner) Shutdown(ctx context.Context) error {
	r.cancelBase()
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runner) ValidateAndNormalizeOptions(opts domain.RunOptions) (domain.RunOptions, error) {
	tag, err := parser.NormalizeHashtag(opts.Hashtag)
	if err != nil {
		return opts, err
	}
	opts.Hashtag = tag

	if len(opts.Platforms) == 0 {
		opts.Platforms = []domain.Platform{domain.PlatformInstagram, domain.PlatformFacebook}
	} else {
		seen := make(map[domain.Platform]bool)
		var cleaned []domain.Platform
		for _, p := range opts.Platforms {
			norm, err := parser.ParsePlatform(string(p))
			if err != nil {
				return opts, err
			}
			if norm == domain.PlatformTikTok {
				return opts, errors.New("TikTok mengambil data langsung dari TikTok (/api/tiktok/fetch), bukan Apify runner")
			}
			if !seen[norm] {
				seen[norm] = true
				cleaned = append(cleaned, norm)
			}
		}
		opts.Platforms = cleaned
	}

	if opts.Limit <= 0 {
		opts.Limit = defaultLimit
	}
	if opts.Limit > maxAllowedLimit {
		return opts, fmt.Errorf("limit maksimal adalah %d video per platform", maxAllowedLimit)
	}

	if opts.Mode == "" {
		opts.Mode = domain.ModeLive
	}
	if opts.Mode != domain.ModeLive && opts.Mode != domain.ModeMock {
		return opts, fmt.Errorf("mode tidak valid: %q (gunakan live atau mock)", opts.Mode)
	}
	if opts.Mode == domain.ModeLive && !r.hasApifyToken {
		return opts, errors.New("APIFY_API_TOKEN belum diisi di .env; aktifkan mode Demo (Mock) atau isi token Apify terlebih dahulu")
	}

	if strings.EqualFold(strings.TrimSpace(opts.IGResultsType), "posts") {
		opts.IGResultsType = "posts"
	} else {
		opts.IGResultsType = "reels"
	}

	return opts, nil
}

func (r *Runner) StartAsync(opts domain.RunOptions) (domain.RunRecord, error) {
	norm, err := r.ValidateAndNormalizeOptions(opts)
	if err != nil {
		return domain.RunRecord{}, err
	}

	rec, err := r.repo.CreateRun(r.baseCtx, norm)
	if err != nil {
		return domain.RunRecord{}, err
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ctx, cancel := context.WithTimeout(r.baseCtx, defaultRunTimeout)
		defer cancel()
		_ = r.executeRun(ctx, rec.ID, norm)
	}()

	return rec, nil
}

func (r *Runner) RunSync(ctx context.Context, opts domain.RunOptions) (domain.RunRecord, domain.HashtagSummary, error) {
	norm, err := r.ValidateAndNormalizeOptions(opts)
	if err != nil {
		return domain.RunRecord{}, domain.HashtagSummary{}, err
	}

	rec, err := r.repo.CreateRun(ctx, norm)
	if err != nil {
		return domain.RunRecord{}, domain.HashtagSummary{}, err
	}

	finalRec := r.executeRun(ctx, rec.ID, norm)
	summary, sumErr := r.repo.GetHashtagSummary(ctx, norm.Hashtag)
	if sumErr != nil {
		return finalRec, domain.HashtagSummary{}, sumErr
	}
	if finalRec.Status == domain.StatusFailed {
		return finalRec, summary, errors.New(finalRec.ErrorMessage)
	}
	return finalRec, summary, nil
}

type platformOutcome struct {
	platform domain.Platform
	res      domain.ProviderResult
	err      error
}

func (r *Runner) persistenceContext(fetchCtx context.Context) (context.Context, context.CancelFunc) {
	if r.baseCtx.Err() != nil {
		return fetchCtx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(fetchCtx), defaultPersistTimeout)
}

func (r *Runner) executeRun(ctx context.Context, runID int64, opts domain.RunOptions) domain.RunRecord {
	providerMap := r.liveProviders
	if opts.Mode == domain.ModeMock {
		providerMap = r.mockProviders
	}

	outcomes := make([]platformOutcome, len(opts.Platforms))
	var wg sync.WaitGroup
	for i, plat := range opts.Platforms {
		p, ok := providerMap[plat]
		if !ok {
			outcomes[i] = platformOutcome{
				platform: plat,
				err:      fmt.Errorf("provider untuk %s tidak tersedia", plat),
			}
			continue
		}
		wg.Add(1)
		go func(idx int, prov provider.HashtagProvider, target domain.Platform) {
			defer wg.Done()
			res, err := prov.FetchVideos(ctx, opts)
			outcomes[idx] = platformOutcome{platform: target, res: res, err: err}
		}(i, p, plat)
	}
	wg.Wait()

	var (
		allVideos      []domain.NormalizedVideo
		rawCount       int
		skippedCount   int
		missingCount   int
		totalViews     int64
		successCount   int
		failCount      int
		messages       []string
	)

	for _, out := range outcomes {
		if out.err != nil {
			failCount++
			messages = append(messages, fmt.Sprintf("%s: %v", out.platform, out.err))
			continue
		}
		successCount++
		rawCount += out.res.RawItemsCount
		skippedCount += out.res.SkippedNonVideo
		allVideos = append(allVideos, out.res.Videos...)
		messages = append(messages, out.res.Notes...)
	}

	for _, v := range allVideos {
		if v.ViewsCount == nil {
			missingCount++
		} else {
			totalViews += *v.ViewsCount
		}
	}

	saveCtx, cancelSave := r.persistenceContext(ctx)
	defer cancelSave()
	if saveErr := r.repo.SaveVideosAndSnapshots(saveCtx, runID, opts.Hashtag, allVideos); saveErr != nil {
		messages = append(messages, fmt.Sprintf("simpan database gagal: %v", saveErr))
		successCount = 0
		failCount++
	}

	status := domain.StatusCompleted
	switch {
	case successCount == 0:
		status = domain.StatusFailed
	case failCount > 0:
		status = domain.StatusPartial
	}

	upd := store.RunCompletionUpdate{
		Status:               status,
		RawItemsCount:        rawCount,
		VideoItemsCount:      len(allVideos),
		SkippedNonVideoCount: skippedCount,
		MissingViewsCount:    missingCount,
		TotalViewsSnapshot:   totalViews,
		ErrorMessage:         strings.Join(messages, "; "),
	}
	finalCtx, cancelFinal := context.WithTimeout(context.Background(), defaultPersistTimeout)
	defer cancelFinal()
	_ = r.repo.FinalizeRun(finalCtx, runID, upd)

	rec, err := r.repo.GetRunByID(finalCtx, runID)
	if err != nil {
		return domain.RunRecord{
			ID:                   runID,
			Hashtag:              opts.Hashtag,
			Platforms:            opts.Platforms,
			LimitPerPlatform:     opts.Limit,
			Mode:                 opts.Mode,
			Status:               status,
			RawItemsCount:        rawCount,
			VideoItemsCount:      len(allVideos),
			SkippedNonVideoCount: skippedCount,
			MissingViewsCount:    missingCount,
			TotalViewsSnapshot:   totalViews,
			ErrorMessage:         upd.ErrorMessage,
		}
	}
	return rec
}
