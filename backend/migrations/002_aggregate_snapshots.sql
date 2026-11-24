CREATE TABLE IF NOT EXISTS aggregate_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	platform TEXT NOT NULL,
	hashtag TEXT NOT NULL,
	total_videos INTEGER NOT NULL,
	total_views INTEGER NOT NULL,
	source TEXT NOT NULL DEFAULT 'tiktok_challenge_detail',
	captured_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_aggregate_snapshots_lookup
	ON aggregate_snapshots (platform, hashtag, id DESC);
