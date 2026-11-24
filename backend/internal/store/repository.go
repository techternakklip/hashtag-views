package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"meta-hashtag-views/internal/domain"
)

const (
	SummaryDisclaimer       = "Metrik dihitung dari TOTAL VIEWS video/Reels publik asli (bukan reach/impressions) pada sampel terindeks sesuai limit penarikan. Video Facebook tanpa angka views publik dipisahkan ke counter missing views."
	TikTokDirectDisclaimer  = "Data asli langsung dari TikTok (total video dan total views kumulatif hashtag tersebut di TikTok)."
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateRun(ctx context.Context, opts domain.RunOptions) (domain.RunRecord, error) {
	platformsJSON, err := json.Marshal(opts.Platforms)
	if err != nil {
		return domain.RunRecord{}, fmt.Errorf("marshal platforms: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO hashtag_runs (hashtag, platforms, limit_per_platform, mode, status, started_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, opts.Hashtag, string(platformsJSON), opts.Limit, string(opts.Mode), string(domain.StatusRunning), now)
	if err != nil {
		return domain.RunRecord{}, fmt.Errorf("insert hashtag_run: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return domain.RunRecord{}, err
	}

	return domain.RunRecord{
		ID:               id,
		Hashtag:          opts.Hashtag,
		Platforms:        opts.Platforms,
		LimitPerPlatform: opts.Limit,
		Mode:             opts.Mode,
		Status:           domain.StatusRunning,
		StartedAt:        now,
	}, nil
}

func (r *Repository) SaveVideosAndSnapshots(ctx context.Context, runID int64, hashtag string, videos []domain.NormalizedVideo) error {
	if len(videos) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	upsertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO videos (
			platform, video_id, hashtag, permalink, author_name, caption, media_type, published_at, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(platform, video_id, hashtag) DO UPDATE SET
			permalink    = CASE WHEN excluded.permalink <> '' THEN excluded.permalink ELSE videos.permalink END,
			author_name  = CASE WHEN excluded.author_name <> '' THEN excluded.author_name ELSE videos.author_name END,
			caption      = CASE WHEN excluded.caption <> '' THEN excluded.caption ELSE videos.caption END,
			media_type   = CASE WHEN excluded.media_type <> '' THEN excluded.media_type ELSE videos.media_type END,
			published_at = CASE WHEN excluded.published_at <> '' THEN excluded.published_at ELSE videos.published_at END,
			last_seen_at = excluded.last_seen_at
	`)
	if err != nil {
		return fmt.Errorf("prepare upsert videos: %w", err)
	}
	defer upsertStmt.Close()

	snapStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO view_snapshots (
			run_id, platform, video_id, hashtag, views_count, views_source, likes_count, comments_count, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert view_snapshots: %w", err)
	}
	defer snapStmt.Close()

	for _, v := range videos {
		if _, err := upsertStmt.ExecContext(
			ctx,
			string(v.Platform),
			v.VideoID,
			hashtag,
			v.Permalink,
			v.AuthorName,
			v.Caption,
			v.MediaType,
			v.PublishedAt,
			now,
			now,
		); err != nil {
			return fmt.Errorf("upsert video %s/%s: %w", v.Platform, v.VideoID, err)
		}

		if _, err := snapStmt.ExecContext(
			ctx,
			runID,
			string(v.Platform),
			v.VideoID,
			hashtag,
			nullableInt64(v.ViewsCount),
			v.ViewsSource,
			nullableInt64(v.LikesCount),
			nullableInt64(v.CommentsCount),
			now,
		); err != nil {
			return fmt.Errorf("insert snapshot %s/%s: %w", v.Platform, v.VideoID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

type RunCompletionUpdate struct {
	Status               domain.RunStatus
	RawItemsCount        int
	VideoItemsCount      int
	SkippedNonVideoCount int
	MissingViewsCount    int
	TotalViewsSnapshot   int64
	ErrorMessage         string
}

func (r *Repository) FinalizeRun(ctx context.Context, runID int64, upd RunCompletionUpdate) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE hashtag_runs
		SET status = ?,
			raw_items_count = ?,
			video_items_count = ?,
			skipped_non_video_count = ?,
			missing_views_count = ?,
			total_views_snapshot = ?,
			error_message = ?,
			finished_at = ?
		WHERE id = ?
	`,
		string(upd.Status),
		upd.RawItemsCount,
		upd.VideoItemsCount,
		upd.SkippedNonVideoCount,
		upd.MissingViewsCount,
		upd.TotalViewsSnapshot,
		upd.ErrorMessage,
		now,
		runID,
	)
	return err
}

func (r *Repository) CountSuccessfulRuns(ctx context.Context, hashtag string) (int, error) {
	return r.CountSuccessfulRunsForPlatform(ctx, hashtag, "")
}

func (r *Repository) CountSuccessfulRunsForPlatform(ctx context.Context, hashtag string, platform domain.Platform) (int, error) {
	whereClause, args := "hashtag = ? AND status IN ('completed', 'partial')", []any{hashtag}
	if platform != "" {
		whereClause += " AND platforms LIKE ?"
		args = append(args, platformJSONPattern(platform))
	}
	var count int
	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hashtag_runs WHERE %s`, whereClause), args...).Scan(&count)
	return count, err
}

func platformJSONPattern(p domain.Platform) string {
	return fmt.Sprintf("%%\"%s\"%%", p)
}

func (r *Repository) GetRunByID(ctx context.Context, id int64) (domain.RunRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, hashtag, platforms, limit_per_platform, mode, status,
			raw_items_count, video_items_count, skipped_non_video_count,
			missing_views_count, total_views_snapshot, error_message, started_at, finished_at
		FROM hashtag_runs
		WHERE id = ?
	`, id)
	return scanRunRecord(row)
}

func (r *Repository) GetHashtagSummary(ctx context.Context, hashtag string) (domain.HashtagSummary, error) {
	return r.GetHashtagSummaryByPlatform(ctx, hashtag, "")
}

func (r *Repository) GetHashtagSummaryByPlatform(ctx context.Context, hashtag string, platform domain.Platform) (domain.HashtagSummary, error) {
	if platform == domain.PlatformTikTok {
		return r.getAggregateSummary(ctx, platform, hashtag)
	}

	summary := domain.HashtagSummary{
		Hashtag: hashtag,
		Platforms: map[domain.Platform]domain.PlatformBreakdown{
			domain.PlatformInstagram: {Platform: domain.PlatformInstagram},
			domain.PlatformFacebook:  {Platform: domain.PlatformFacebook},
			domain.PlatformTikTok:    {Platform: domain.PlatformTikTok},
		},
		RunHistory: []domain.RunRecord{},
		Disclaimer: SummaryDisclaimer,
	}

	whereClause := "v.hashtag = ?"
	args := []any{hashtag}
	if platform != "" {
		whereClause += " AND v.platform = ?"
		args = append(args, string(platform))
	}

	query := fmt.Sprintf(`
		WITH latest AS (
			SELECT
				v.platform,
				v.video_id,
				s.views_count,
				s.fetched_at,
				ROW_NUMBER() OVER (
					PARTITION BY v.platform, v.video_id
					ORDER BY s.fetched_at DESC, s.id DESC
				) AS rn
			FROM videos v
			LEFT JOIN view_snapshots s
				ON v.platform = s.platform
				AND v.video_id = s.video_id
				AND v.hashtag = s.hashtag
			WHERE %s
		)
		SELECT
			platform,
			COUNT(*) AS total_videos,
			SUM(CASE WHEN views_count IS NOT NULL THEN 1 ELSE 0 END) AS videos_with_views,
			SUM(CASE WHEN views_count IS NULL THEN 1 ELSE 0 END) AS videos_missing_views,
			COALESCE(SUM(views_count), 0) AS total_views,
			COALESCE(MAX(fetched_at), '') AS last_fetched_at
		FROM latest
		WHERE rn = 1
		GROUP BY platform
	`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return summary, fmt.Errorf("query hashtag summary: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			plat        string
			totalVideos int
			withViews   int
			missing     int
			totalViews  int64
			lastFetched string
		)
		if err := rows.Scan(&plat, &totalVideos, &withViews, &missing, &totalViews, &lastFetched); err != nil {
			return summary, err
		}
		p := domain.Platform(plat)
		summary.Platforms[p] = domain.PlatformBreakdown{
			Platform:           p,
			TotalViews:         totalViews,
			TotalVideos:        totalVideos,
			VideosWithViews:    withViews,
			VideosMissingViews: missing,
		}
		summary.GrandTotalViews += totalViews
		summary.TotalVideos += totalVideos
		summary.VideosWithViews += withViews
		summary.VideosMissingViews += missing
		if lastFetched > summary.LastUpdatedAt {
			summary.LastUpdatedAt = lastFetched
		}
	}
	if err := rows.Err(); err != nil {
		return summary, err
	}

	history, err := r.listRunHistory(ctx, hashtag, platform, 20)
	if err != nil {
		return summary, err
	}
	summary.RunHistory = history

	if err := r.populateDelta(ctx, &summary, platform); err != nil {
		return summary, err
	}

	return summary, nil
}

func (r *Repository) populateDelta(ctx context.Context, summary *domain.HashtagSummary, platform domain.Platform) error {
	whereClause, args := "hashtag = ? AND status IN ('completed', 'partial')", []any{summary.Hashtag}
	if platform != "" {
		whereClause += " AND platforms LIKE ?"
		args = append(args, platformJSONPattern(platform))
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id
		FROM hashtag_runs
		WHERE %s
		ORDER BY id DESC
		LIMIT 2
	`, whereClause), args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	var runIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		runIDs = append(runIDs, id)
	}
	if len(runIDs) < 2 {
		return nil
	}

	prevRunID := runIDs[1]
	prevTotals, prevVideos, err := r.cumulativeTotalsAsOfRun(ctx, summary.Hashtag, prevRunID, platform)
	if err != nil {
		return err
	}

	dViews := summary.GrandTotalViews - prevTotals
	dVideos := summary.TotalVideos - prevVideos
	summary.DeltaViews = &dViews
	summary.DeltaVideos = &dVideos
	summary.PreviousRunID = &prevRunID
	return nil
}

func (r *Repository) cumulativeTotalsAsOfRun(ctx context.Context, hashtag string, maxRunID int64, platform domain.Platform) (int64, int, error) {
	var totalViews int64
	var totalVideos int
	whereClause := "hashtag = ? AND run_id <= ?"
	args := []any{hashtag, maxRunID}
	if platform != "" {
		whereClause += " AND platform = ?"
		args = append(args, string(platform))
	}

	query := fmt.Sprintf(`
		WITH latest_as_of AS (
			SELECT
				platform,
				video_id,
				views_count,
				ROW_NUMBER() OVER (
					PARTITION BY platform, video_id
					ORDER BY fetched_at DESC, id DESC
				) AS rn
			FROM view_snapshots
			WHERE %s
		)
		SELECT
			COALESCE(SUM(views_count), 0),
			COUNT(*)
		FROM latest_as_of
		WHERE rn = 1
	`, whereClause)

	err := r.db.QueryRowContext(ctx, query, args...).Scan(&totalViews, &totalVideos)
	return totalViews, totalVideos, err
}

func (r *Repository) listRunHistory(ctx context.Context, hashtag string, platform domain.Platform, limit int) ([]domain.RunRecord, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if platform != "" {
		like := fmt.Sprintf("%%\"%s\"%%", platform)
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				hr.id,
				hr.hashtag,
				hr.platforms,
				hr.limit_per_platform,
				hr.mode,
				hr.status,
				hr.raw_items_count,
				COALESCE(ps.video_count, 0),
				hr.skipped_non_video_count,
				COALESCE(ps.missing_count, 0),
				COALESCE(ps.total_views, 0),
				hr.error_message,
				hr.started_at,
				hr.finished_at
			FROM hashtag_runs hr
			LEFT JOIN (
				SELECT
					run_id,
					COUNT(*) AS video_count,
					SUM(CASE WHEN views_count IS NULL THEN 1 ELSE 0 END) AS missing_count,
					COALESCE(SUM(views_count), 0) AS total_views
				FROM view_snapshots
				WHERE hashtag = ? AND platform = ?
				GROUP BY run_id
			) ps ON ps.run_id = hr.id
			WHERE hr.hashtag = ? AND hr.platforms LIKE ?
			ORDER BY hr.id DESC
			LIMIT ?
		`, hashtag, string(platform), hashtag, like, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, hashtag, platforms, limit_per_platform, mode, status,
				raw_items_count, video_items_count, skipped_non_video_count,
				missing_views_count, total_views_snapshot, error_message, started_at, finished_at
			FROM hashtag_runs
			WHERE hashtag = ?
			ORDER BY id DESC
			LIMIT ?
		`, hashtag, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.RunRecord
	for rows.Next() {
		rec, err := scanRunRecord(rows)
		if err != nil {
			return nil, err
		}
		if platform != "" {
			rec.Platforms = []domain.Platform{platform}
		}
		list = append(list, rec)
	}
	if list == nil {
		list = []domain.RunRecord{}
	}
	return list, rows.Err()
}

func (r *Repository) ListHashtagVideos(ctx context.Context, hashtag string, opts domain.VideoQueryOptions) ([]domain.VideoViewRow, error) {
	var whereClauses []string
	args := []any{hashtag}
	whereClauses = append(whereClauses, "rn = 1")

	if opts.Platform == string(domain.PlatformInstagram) ||
		opts.Platform == string(domain.PlatformFacebook) ||
		opts.Platform == string(domain.PlatformTikTok) {
		whereClauses = append(whereClauses, "platform = ?")
		args = append(args, opts.Platform)
	}

	if q := strings.TrimSpace(opts.Search); q != "" {
		whereClauses = append(whereClauses, "(LOWER(caption) LIKE ? OR LOWER(author_name) LIKE ? OR LOWER(video_id) LIKE ?)")
		like := "%" + strings.ToLower(q) + "%"
		args = append(args, like, like, like)
	}

	orderBy := "CASE WHEN views_count IS NULL THEN 1 ELSE 0 END ASC, views_count DESC, last_fetched_at DESC"
	switch opts.Sort {
	case "views_asc":
		orderBy = "CASE WHEN views_count IS NULL THEN 1 ELSE 0 END ASC, views_count ASC, last_fetched_at DESC"
	case "published_desc":
		orderBy = "published_at DESC, views_count DESC"
	}

	limit := opts.Limit
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		WITH latest_videos AS (
			SELECT
				v.platform,
				v.hashtag,
				v.video_id,
				v.permalink,
				v.author_name,
				v.caption,
				v.media_type,
				v.published_at,
				s.views_count,
				COALESCE(s.views_source, '') AS views_source,
				s.likes_count,
				s.comments_count,
				v.first_seen_at,
				v.last_seen_at,
				COALESCE(s.fetched_at, v.last_seen_at) AS last_fetched_at,
				ROW_NUMBER() OVER (
					PARTITION BY v.platform, v.video_id
					ORDER BY s.fetched_at DESC, s.id DESC
				) AS rn
			FROM videos v
			LEFT JOIN view_snapshots s
				ON v.platform = s.platform
				AND v.video_id = s.video_id
				AND v.hashtag = s.hashtag
			WHERE v.hashtag = ?
		)
		SELECT
			platform, hashtag, video_id, permalink, author_name, caption, media_type, published_at,
			views_count, views_source, likes_count, comments_count, first_seen_at, last_seen_at, last_fetched_at
		FROM latest_videos
		WHERE %s
		ORDER BY %s
		LIMIT ?
	`, strings.Join(whereClauses, " AND "), orderBy)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query hashtag videos: %w", err)
	}
	defer rows.Close()

	var result []domain.VideoViewRow
	for rows.Next() {
		var (
			item                      domain.VideoViewRow
			plat                      string
			views, likes, comments    sql.NullInt64
		)
		if err := rows.Scan(
			&plat,
			&item.Hashtag,
			&item.VideoID,
			&item.Permalink,
			&item.AuthorName,
			&item.Caption,
			&item.MediaType,
			&item.PublishedAt,
			&views,
			&item.ViewsSource,
			&likes,
			&comments,
			&item.FirstSeenAt,
			&item.LastSeenAt,
			&item.LastFetchedAt,
		); err != nil {
			return nil, err
		}
		item.Platform = domain.Platform(plat)
		item.ViewsCount = nullInt64Ptr(views)
		item.LikesCount = nullInt64Ptr(likes)
		item.CommentsCount = nullInt64Ptr(comments)
		result = append(result, item)
	}
	if result == nil {
		result = []domain.VideoViewRow{}
	}
	return result, rows.Err()
}

func (r *Repository) ListTrackedHashtags(ctx context.Context) ([]domain.TrackedHashtag, error) {
	return r.ListTrackedHashtagsByPlatform(ctx, "")
}

func (r *Repository) ListTrackedHashtagsByPlatform(ctx context.Context, platform domain.Platform) ([]domain.TrackedHashtag, error) {
	if platform == domain.PlatformTikTok {
		return r.listAggregateTrackedHashtags(ctx, platform)
	}

	var (
		runFilter   string
		videoFilter string
		args        []any
	)
	if platform != "" {
		like := platformJSONPattern(platform)
		runFilter = "WHERE platforms LIKE ?"
		videoFilter = "WHERE platform = ?"
		args = []any{like, string(platform), string(platform), like}
	}

	query := fmt.Sprintf(`
		WITH all_tags AS (
			SELECT DISTINCT hashtag FROM hashtag_runs %s
			UNION
			SELECT DISTINCT hashtag FROM videos %s
		),
		latest_videos AS (
			SELECT
				v.hashtag,
				v.platform,
				v.video_id,
				s.views_count,
				ROW_NUMBER() OVER (
					PARTITION BY v.hashtag, v.platform, v.video_id
					ORDER BY s.fetched_at DESC, s.id DESC
				) AS rn
			FROM videos v
			LEFT JOIN view_snapshots s
				ON v.platform = s.platform
				AND v.video_id = s.video_id
				AND v.hashtag = s.hashtag
			%s
		),
		agg AS (
			SELECT
				hashtag,
				COALESCE(SUM(views_count), 0) AS grand_total_views,
				COUNT(*) AS total_videos,
				SUM(CASE WHEN views_count IS NULL THEN 1 ELSE 0 END) AS missing_views,
				COALESCE(SUM(CASE WHEN platform = 'instagram' THEN views_count ELSE 0 END), 0) AS ig_views,
				COALESCE(SUM(CASE WHEN platform = 'facebook' THEN views_count ELSE 0 END), 0) AS fb_views,
				COALESCE(SUM(CASE WHEN platform = 'tiktok' THEN views_count ELSE 0 END), 0) AS tt_views
			FROM latest_videos
			WHERE rn = 1
			GROUP BY hashtag
		),
		latest_run AS (
			SELECT
				hashtag,
				started_at,
				status,
				mode,
				ROW_NUMBER() OVER (PARTITION BY hashtag ORDER BY id DESC) AS rn
			FROM hashtag_runs
			%s
		)
		SELECT
			t.hashtag,
			COALESCE(a.grand_total_views, 0),
			COALESCE(a.total_videos, 0),
			COALESCE(a.missing_views, 0),
			COALESCE(a.ig_views, 0),
			COALESCE(a.fb_views, 0),
			COALESCE(a.tt_views, 0),
			COALESCE(r.started_at, ''),
			COALESCE(r.status, ''),
			COALESCE(r.mode, '')
		FROM all_tags t
		LEFT JOIN agg a ON a.hashtag = t.hashtag
		LEFT JOIN latest_run r ON r.hashtag = t.hashtag AND r.rn = 1
		ORDER BY COALESCE(r.started_at, '') DESC, COALESCE(a.grand_total_views, 0) DESC, t.hashtag ASC
	`, runFilter, videoFilter, strings.Replace(videoFilter, "platform", "v.platform", 1), runFilter)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tracked hashtags: %w", err)
	}
	defer rows.Close()

	var tags []domain.TrackedHashtag
	for rows.Next() {
		var item domain.TrackedHashtag
		if err := rows.Scan(
			&item.Hashtag,
			&item.GrandTotalViews,
			&item.TotalVideos,
			&item.VideosMissingViews,
			&item.InstagramViews,
			&item.FacebookViews,
			&item.TikTokViews,
			&item.LastRunAt,
			&item.LastRunStatus,
			&item.LastRunMode,
		); err != nil {
			return nil, err
		}
		tags = append(tags, item)
	}
	if tags == nil {
		tags = []domain.TrackedHashtag{}
	}
	return tags, rows.Err()
}

func (r *Repository) SaveAggregateSnapshot(
	ctx context.Context,
	platform domain.Platform,
	hashtag string,
	totalVideos, totalViews int64,
	source string,
	capturedAt time.Time,
) (domain.AggregateSnapshot, error) {
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	ts := capturedAt.UTC().Format(time.RFC3339Nano)
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO aggregate_snapshots (platform, hashtag, total_videos, total_views, source, captured_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, string(platform), hashtag, totalVideos, totalViews, source, ts)
	if err != nil {
		return domain.AggregateSnapshot{}, fmt.Errorf("insert aggregate_snapshot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.AggregateSnapshot{}, err
	}
	return domain.AggregateSnapshot{
		ID:          id,
		Platform:    platform,
		Hashtag:     hashtag,
		TotalVideos: totalVideos,
		TotalViews:  totalViews,
		Source:      source,
		CapturedAt:  ts,
	}, nil
}

func (r *Repository) ListAggregateSnapshots(ctx context.Context, platform domain.Platform, hashtag string, limit int) ([]domain.AggregateSnapshot, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, platform, hashtag, total_videos, total_views, source, captured_at
		FROM aggregate_snapshots
		WHERE platform = ? AND hashtag = ?
		ORDER BY id DESC
		LIMIT ?
	`, string(platform), hashtag, limit)
	if err != nil {
		return nil, fmt.Errorf("list aggregate snapshots: %w", err)
	}
	defer rows.Close()

	var list []domain.AggregateSnapshot
	for rows.Next() {
		var (
			s    domain.AggregateSnapshot
			plat string
		)
		if err := rows.Scan(&s.ID, &plat, &s.Hashtag, &s.TotalVideos, &s.TotalViews, &s.Source, &s.CapturedAt); err != nil {
			return nil, err
		}
		s.Platform = domain.Platform(plat)
		list = append(list, s)
	}
	if list == nil {
		list = []domain.AggregateSnapshot{}
	}
	return list, rows.Err()
}

func (r *Repository) getAggregateSummary(ctx context.Context, platform domain.Platform, hashtag string) (domain.HashtagSummary, error) {
	summary := domain.HashtagSummary{
		Hashtag: hashtag,
		Platforms: map[domain.Platform]domain.PlatformBreakdown{
			domain.PlatformInstagram: {Platform: domain.PlatformInstagram},
			domain.PlatformFacebook:  {Platform: domain.PlatformFacebook},
			domain.PlatformTikTok:    {Platform: domain.PlatformTikTok},
		},
		RunHistory:      []domain.RunRecord{},
		Disclaimer:      TikTokDirectDisclaimer,
		IsAggregateOnly: true,
	}

	snapshots, err := r.ListAggregateSnapshots(ctx, platform, hashtag, 20)
	if err != nil {
		return summary, err
	}
	if len(snapshots) == 0 {
		return summary, nil
	}

	latest := snapshots[0]
	summary.GrandTotalViews = latest.TotalViews
	summary.TotalVideos = int(latest.TotalVideos)
	summary.VideosWithViews = int(latest.TotalVideos)
	summary.LastUpdatedAt = latest.CapturedAt
	summary.Platforms[platform] = domain.PlatformBreakdown{
		Platform:        platform,
		TotalViews:      latest.TotalViews,
		TotalVideos:     int(latest.TotalVideos),
		VideosWithViews: int(latest.TotalVideos),
	}

	if len(snapshots) >= 2 {
		prev := snapshots[1]
		dViews := latest.TotalViews - prev.TotalViews
		dVideos := int(latest.TotalVideos - prev.TotalVideos)
		prevID := prev.ID
		summary.DeltaViews = &dViews
		summary.DeltaVideos = &dVideos
		summary.PreviousRunID = &prevID
	}

	history := make([]domain.RunRecord, 0, len(snapshots))
	for _, s := range snapshots {
		history = append(history, domain.RunRecord{
			ID:                 s.ID,
			Hashtag:            s.Hashtag,
			Platforms:          []domain.Platform{s.Platform},
			Mode:               domain.ModeDirect,
			Status:             domain.StatusCompleted,
			VideoItemsCount:    int(s.TotalVideos),
			TotalViewsSnapshot: s.TotalViews,
			StartedAt:          s.CapturedAt,
			FinishedAt:         s.CapturedAt,
		})
	}
	summary.RunHistory = history
	return summary, nil
}

func (r *Repository) listAggregateTrackedHashtags(ctx context.Context, platform domain.Platform) ([]domain.TrackedHashtag, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH latest AS (
			SELECT
				hashtag,
				total_videos,
				total_views,
				captured_at,
				ROW_NUMBER() OVER (PARTITION BY hashtag ORDER BY id DESC) AS rn
			FROM aggregate_snapshots
			WHERE platform = ?
		)
		SELECT hashtag, total_views, total_videos, captured_at
		FROM latest
		WHERE rn = 1
		ORDER BY captured_at DESC, total_views DESC, hashtag ASC
	`, string(platform))
	if err != nil {
		return nil, fmt.Errorf("list aggregate tracked hashtags: %w", err)
	}
	defer rows.Close()

	var tags []domain.TrackedHashtag
	for rows.Next() {
		var (
			tag        string
			views      int64
			videos     int64
			capturedAt string
		)
		if err := rows.Scan(&tag, &views, &videos, &capturedAt); err != nil {
			return nil, err
		}
		item := domain.TrackedHashtag{
			Hashtag:         tag,
			GrandTotalViews: views,
			TotalVideos:     int(videos),
			LastRunAt:       capturedAt,
			LastRunStatus:   string(domain.StatusCompleted),
			LastRunMode:     string(domain.ModeDirect),
		}
		if platform == domain.PlatformTikTok {
			item.TikTokViews = views
		}
		tags = append(tags, item)
	}
	if tags == nil {
		tags = []domain.TrackedHashtag{}
	}
	return tags, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRunRecord(s rowScanner) (domain.RunRecord, error) {
	var (
		rec           domain.RunRecord
		platformsJSON string
		mode          string
		status        string
	)
	if err := s.Scan(
		&rec.ID,
		&rec.Hashtag,
		&platformsJSON,
		&rec.LimitPerPlatform,
		&mode,
		&status,
		&rec.RawItemsCount,
		&rec.VideoItemsCount,
		&rec.SkippedNonVideoCount,
		&rec.MissingViewsCount,
		&rec.TotalViewsSnapshot,
		&rec.ErrorMessage,
		&rec.StartedAt,
		&rec.FinishedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RunRecord{}, err
		}
		return domain.RunRecord{}, err
	}
	rec.Mode = domain.RunMode(mode)
	rec.Status = domain.RunStatus(status)
	_ = json.Unmarshal([]byte(platformsJSON), &rec.Platforms)
	if rec.Platforms == nil {
		rec.Platforms = []domain.Platform{}
	}
	return rec, nil
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	val := v.Int64
	return &val
}
