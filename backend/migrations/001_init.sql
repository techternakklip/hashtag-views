CREATE TABLE IF NOT EXISTS hashtag_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	hashtag TEXT NOT NULL,
	platforms TEXT NOT NULL,
	limit_per_platform INTEGER NOT NULL,
	mode TEXT NOT NULL,
	status TEXT NOT NULL,
	raw_items_count INTEGER NOT NULL DEFAULT 0,
	video_items_count INTEGER NOT NULL DEFAULT 0,
	skipped_non_video_count INTEGER NOT NULL DEFAULT 0,
	missing_views_count INTEGER NOT NULL DEFAULT 0,
	total_views_snapshot INTEGER NOT NULL DEFAULT 0,
	error_message TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_hashtag_runs_tag_started
	ON hashtag_runs (hashtag, id DESC);

CREATE TABLE IF NOT EXISTS videos (
	platform TEXT NOT NULL,
	video_id TEXT NOT NULL,
	hashtag TEXT NOT NULL,
	permalink TEXT NOT NULL,
	author_name TEXT NOT NULL DEFAULT '',
	caption TEXT NOT NULL DEFAULT '',
	media_type TEXT NOT NULL DEFAULT 'video',
	published_at TEXT NOT NULL DEFAULT '',
	first_seen_at TEXT NOT NULL,
	last_seen_at TEXT NOT NULL,
	PRIMARY KEY (platform, video_id, hashtag)
);

CREATE INDEX IF NOT EXISTS idx_videos_hashtag_platform
	ON videos (hashtag, platform);

CREATE TABLE IF NOT EXISTS view_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id INTEGER NOT NULL,
	platform TEXT NOT NULL,
	video_id TEXT NOT NULL,
	hashtag TEXT NOT NULL,
	views_count INTEGER,
	views_source TEXT NOT NULL DEFAULT '',
	likes_count INTEGER,
	comments_count INTEGER,
	fetched_at TEXT NOT NULL,
	FOREIGN KEY (run_id) REFERENCES hashtag_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_view_snapshots_lookup
	ON view_snapshots (hashtag, platform, video_id, fetched_at DESC, id DESC);
