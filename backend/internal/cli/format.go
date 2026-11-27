package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"meta-hashtag-views/internal/domain"
)

func printSummary(out io.Writer, s domain.HashtagSummary) {
	fmt.Fprintf(out, "\n=== RINGKASAN TOTAL REAL VIEWS: #%s ===\n", s.Hashtag)
	fmt.Fprintf(out, "Grand Total Views : %s views\n", FormatDots(s.GrandTotalViews))
	fmt.Fprintf(out, "Total Video       : %d video (%d ada views, %d views disembunyikan)\n",
		s.TotalVideos, s.VideosWithViews, s.VideosMissingViews)
	if s.DeltaViews != nil && s.DeltaVideos != nil {
		deltaSign := ""
		if *s.DeltaViews > 0 {
			deltaSign = "+"
		}
		fmt.Fprintf(out, "Delta vs Run Lalu : %s%s views (%+d video baru)\n", deltaSign, FormatDots(*s.DeltaViews), *s.DeltaVideos)
	}
	fmt.Fprintln(out, "")

	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PLATFORM\tTOTAL VIEWS\tTOTAL VIDEO\tADA VIEWS\tMISSING VIEWS")
	for _, p := range domain.AllPlatforms {
		b := s.Platforms[p]
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\n", p, FormatDots(b.TotalViews), b.TotalVideos, b.VideosWithViews, b.VideosMissingViews)
	}
	_ = tw.Flush()
	fmt.Fprintf(out, "\nCatatan: %s\n", s.Disclaimer)
}

func printTopVideos(out io.Writer, videos []domain.VideoViewRow) {
	if len(videos) == 0 {
		return
	}
	fmt.Fprintln(out, "\nTop 10 Video berdasarkan Views:")
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PLATFORM\tVIEWS\tSOURCE\tAUTHOR\tPERMALINK")
	for _, v := range videos {
		viewsStr := "N/A (hidden)"
		if v.ViewsCount != nil {
			viewsStr = FormatDots(*v.ViewsCount)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", v.Platform, viewsStr, v.ViewsSource, v.AuthorName, v.Permalink)
	}
	_ = tw.Flush()
}

func FormatDots(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append(parts, s[len(s)-3:])
		s = s[:len(s)-3]
	}
	parts = append(parts, s)
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	out := strings.Join(parts, ".")
	if neg {
		return "-" + out
	}
	return out
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `Meta Hashtag Views CLI (Go)

Usage:
  meta-views serve  [--host 127.0.0.1] [--port 8080]
  meta-views fetch  --hashtag <tag> [--platforms ig,fb] [--limit 50] [--mock] [--enrich-fb] [--ig-type reels]
  meta-views report --hashtag <tag>
  meta-views export --hashtag <tag> [--format csv|json] [--out ./exports]`)
}
