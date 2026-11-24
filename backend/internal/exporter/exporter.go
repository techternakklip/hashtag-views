package exporter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"meta-hashtag-views/internal/domain"
)

const utf8BOM = "\xEF\xBB\xBF"

type JSONReport struct {
	ExportedAt string                `json:"exportedAt"`
	Summary    domain.HashtagSummary `json:"summary"`
	Videos     []domain.VideoViewRow `json:"videos"`
}

func ToCSV(videos []domain.VideoViewRow) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(utf8BOM)

	w := csv.NewWriter(&buf)
	header := []string{
		"platform",
		"hashtag",
		"video_id",
		"views_count",
		"views_source",
		"media_type",
		"author_name",
		"published_at",
		"permalink",
		"likes_count",
		"comments_count",
		"last_fetched_at",
		"caption",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, v := range videos {
		row := []string{
			string(v.Platform),
			v.Hashtag,
			v.VideoID,
			formatOptionalInt(v.ViewsCount),
			v.ViewsSource,
			v.MediaType,
			sanitizeSpreadsheetCell(v.AuthorName),
			v.PublishedAt,
			v.Permalink,
			formatOptionalInt(v.LikesCount),
			formatOptionalInt(v.CommentsCount),
			v.LastFetchedAt,
			sanitizeSpreadsheetCell(compactNewlines(v.Caption)),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ToAggregateCSV(snapshots []domain.AggregateSnapshot) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(utf8BOM)

	w := csv.NewWriter(&buf)
	header := []string{"snapshot_id", "platform", "hashtag", "total_videos", "total_views", "source", "captured_at"}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, s := range snapshots {
		row := []string{
			strconv.FormatInt(s.ID, 10),
			string(s.Platform),
			s.Hashtag,
			strconv.FormatInt(s.TotalVideos, 10),
			strconv.FormatInt(s.TotalViews, 10),
			s.Source,
			s.CapturedAt,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ToJSONReport(summary domain.HashtagSummary, videos []domain.VideoViewRow) ([]byte, error) {
	if videos == nil {
		videos = []domain.VideoViewRow{}
	}
	report := JSONReport{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:    summary,
		Videos:     videos,
	}
	return json.MarshalIndent(report, "", "  ")
}

func formatOptionalInt(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func compactNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// sanitizeSpreadsheetCell prevents CSV formula injection when opened in Excel/Sheets.
func sanitizeSpreadsheetCell(s string) string {
	if s == "" {
		return ""
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	default:
		return s
	}
}
