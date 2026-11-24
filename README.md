# Hashtag Views Tracker

Full-stack application (**Go 1.27 backend + pure-Go SQLite + CLI** and **React + TypeScript + Vite + Tailwind CSS dashboard**) for tracking **real total video views (not reach or impressions)** per `#hashtag` across **TikTok**, **Instagram**, and **Facebook**.

---

## How It Works

### TikTok
* **Direct Challenge Fetch:** Queries TikTok's challenge detail endpoint server-side to read unrounded `statsV2.videoCount` and `statsV2.viewCount`.
* **SQLite Cache & Upstream Pacing:** Stores cumulative snapshots in SQLite with a `5`-minute cache TTL (supports force refresh via **Perbarui**), `1.2s` minimum spacing between consecutive upstream requests, and automatic cooldown handling if TikTok returns HTTP `429` (`Retry-After`).

### Instagram & Facebook
* **Per-Video Extraction:** Neither Instagram nor Facebook exposes global cumulative hashtag view totals, so the backend fetches indexed public videos/Reels through Apify REST API v2 actors (`apify/instagram-hashtag-scraper`, `apify/facebook-hashtag-scraper`, and optional `apify/facebook-reels-scraper`).
* **Deduplication & Growth Deltas:** Deduplicates `(platform, video_id, hashtag)` in SQLite and computes run-over-run delta metrics without double-counting repeat videos.
* **Local Simulation Mode:** Includes a deterministic `mock` provider when `ALLOW_MOCK_MODE=true` for offline testing and demos without Apify credits.

---

## Environment Configuration & Feature Flags

Copy the example configuration file at the repository root:

```bash
cp .env.example .env
```

### Variables
| Variable | Default | Description |
| :--- | :--- | :--- |
| `ENABLED_PLATFORMS` | `instagram,facebook,tiktok` | Comma-separated enabled platforms (`ig`/`instagram`, `fb`/`facebook`, `tt`/`tiktok`). Disabled platforms return `403` from the API and are hidden in the UI. |
| `ALLOW_MOCK_MODE` | `true` | Allows local simulation runs. Set to `false` in production so searches never fall back to simulated data. |
| `APIFY_API_TOKEN` | _(empty)_ | Apify API token for live Instagram and Facebook scraping. |
| `PORT` | `8080` | HTTP server port for `serve`. |
| `DB_PATH` | `./data/hashtag-views.db` | SQLite database file path. |
| `FRONTEND_ORIGIN` | `http://localhost:5173` | Allowed CORS origin during separate frontend development. |
| `APIFY_IG_HASHTAG_ACTOR` | `apify/instagram-hashtag-scraper` | Override for the Instagram hashtag scraper actor ID. |
| `APIFY_FB_HASHTAG_ACTOR` | `apify/facebook-hashtag-scraper` | Override for the Facebook hashtag scraper actor ID. |
| `APIFY_FB_REELS_ACTOR` | `apify/facebook-reels-scraper` | Override for the optional Facebook Reels views enricher actor ID. |

### Local Development vs. Production

* **Local Development (all platforms + simulation enabled):**
  ```bash
  ENABLED_PLATFORMS=instagram,facebook,tiktok ALLOW_MOCK_MODE=true ./bin/meta-views serve --port 8080
  ```
* **Production (TikTok-only, simulation disabled, platform switcher hidden):**
  ```bash
  ENABLED_PLATFORMS=tiktok ALLOW_MOCK_MODE=false DB_PATH=/var/data/hashtag-views.db ./bin/meta-views serve --port 8080
  ```

---

## Quick Start

### 1. Run Backend & Frontend Verification
```bash
make vet
make test
```

### 2. Run in Development Mode (`:8080` API + `:5173` Vite UI)
```bash
make dev
```
Or start each process separately:
```bash
make dev-backend
make dev-frontend
```

### 3. Build Single-Binary Production Server (`:8080`)
```bash
make build
./bin/meta-views serve --port 8080
```

---

## CLI Usage

```bash
# Run local mock collection for Instagram & Facebook
make fetch HASHTAG=gta LIMIT=30 MOCK=1

# Run live Instagram & Facebook collection (requires APIFY_API_TOKEN)
make fetch HASHTAG=gta LIMIT=50

# Enrich Facebook videos with hidden feed view counts
make fetch HASHTAG=gta LIMIT=50 ENRICH_FB=1

# Print summary & top 10 videos from SQLite
make report HASHTAG=gta

# Export CSV (Excel UTF-8 BOM) or JSON
make export HASHTAG=gta FORMAT=csv
make export HASHTAG=gta FORMAT=json
```

---

## Data Transparency

1. **Real Views, Not Reach:** Totals only sum genuine video/Reel play/view counts (`videoPlayCount`, `playCount`, `videoViewCount`, or TikTok `statsV2.viewCount`).
2. **Hidden Counts Separated:** Facebook videos that omit public view counts are not estimated; they are tracked separately under `videosMissingViews` and can optionally be enriched via `apify/facebook-reels-scraper`.
3. **Excel-Safe Exports:** CSV exports prepend a UTF-8 BOM (`\xEF\xBB\xBF`) and neutralize leading spreadsheet formula characters (`=`, `+`, `-`, `@`, `\t`, `\r`).
