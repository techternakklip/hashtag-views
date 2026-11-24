package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrTikTokHashtagNotFound = errors.New("Hashtag ini tidak ditemukan di TikTok. Periksa kembali ejaannya.")

type TikTokChallengeStats struct {
	Hashtag     string
	TotalVideos int64
	TotalViews  int64
	CapturedAt  time.Time
}

type challengeDetailPayload struct {
	StatusCode    int `json:"statusCode"`
	ChallengeInfo struct {
		Challenge struct {
			Title string `json:"title"`
		} `json:"challenge"`
		StatsV2 struct {
			VideoCount json.RawMessage `json:"videoCount"`
			ViewCount  json.RawMessage `json:"viewCount"`
		} `json:"statsV2"`
	} `json:"challengeInfo"`
	Extra struct {
		Now int64 `json:"now"`
	} `json:"extra"`
}

func ParseTikTokChallengeDetail(raw []byte) (TikTokChallengeStats, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return TikTokChallengeStats{}, errors.New("respons dari TikTok kosong")
	}

	var payload challengeDetailPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return TikTokChallengeStats{}, errors.New("respons dari TikTok bukan JSON yang valid; coba lagi beberapa saat")
	}

	if payload.StatusCode == 10205 {
		return TikTokChallengeStats{}, ErrTikTokHashtagNotFound
	}
	if payload.StatusCode != 0 {
		return TikTokChallengeStats{}, fmt.Errorf("TikTok belum bisa memberikan data (kode %d). Coba lagi beberapa saat.", payload.StatusCode)
	}

	videos, okV := parseFlexInt64(payload.ChallengeInfo.StatsV2.VideoCount)
	views, okVw := parseFlexInt64(payload.ChallengeInfo.StatsV2.ViewCount)
	if !okV || !okVw {
		return TikTokChallengeStats{}, errors.New("data jumlah video atau views tidak tersedia dari TikTok untuk hashtag ini")
	}

	tag := ""
	if payload.ChallengeInfo.Challenge.Title != "" {
		if norm, err := NormalizeHashtag(payload.ChallengeInfo.Challenge.Title); err == nil {
			tag = norm
		}
	}

	capturedAt := time.Now().UTC()
	if payload.Extra.Now > 0 {
		capturedAt = time.UnixMilli(payload.Extra.Now).UTC()
	}

	return TikTokChallengeStats{
		Hashtag:     tag,
		TotalVideos: videos,
		TotalViews:  views,
		CapturedAt:  capturedAt,
	}, nil
}

func parseFlexInt64(raw json.RawMessage) (int64, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, false
	}
	s = strings.Trim(s, `"`)
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
