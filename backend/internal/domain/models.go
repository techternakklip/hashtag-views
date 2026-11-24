package domain

type Platform string

const (
	PlatformInstagram Platform = "instagram"
	PlatformFacebook  Platform = "facebook"
	PlatformTikTok    Platform = "tiktok"
)

var AllPlatforms = []Platform{
	PlatformInstagram,
	PlatformFacebook,
	PlatformTikTok,
}

type RunMode string

const (
	ModeLive   RunMode = "live"
	ModeMock   RunMode = "mock"
	ModeDirect RunMode = "direct"
)

type AggregateSnapshot struct {
	ID          int64    `json:"id"`
	Platform    Platform `json:"platform"`
	Hashtag     string   `json:"hashtag"`
	TotalVideos int64    `json:"totalVideos"`
	TotalViews  int64    `json:"totalViews"`
	Source      string   `json:"source"`
	CapturedAt  string   `json:"capturedAt"`
}

type RunStatus string

const (
	StatusRunning   RunStatus = "running"
	StatusCompleted RunStatus = "completed"
	StatusPartial   RunStatus = "partial"
	StatusFailed    RunStatus = "failed"
)

type RunOptions struct {
	Hashtag       string     `json:"hashtag"`
	Platforms     []Platform `json:"platforms"`
	Limit         int        `json:"limit"`
	Mode          RunMode    `json:"mode"`
	EnrichFB      bool       `json:"enrichFb"`
	IGResultsType string     `json:"igResultsType"`
}

type NormalizedVideo struct {
	Platform      Platform `json:"platform"`
	VideoID       string   `json:"videoId"`
	Hashtag       string   `json:"hashtag"`
	Permalink     string   `json:"permalink"`
	AuthorName    string   `json:"authorName"`
	Caption       string   `json:"caption"`
	MediaType     string   `json:"mediaType"`
	PublishedAt   string   `json:"publishedAt"`
	ViewsCount    *int64   `json:"viewsCount"`
	ViewsSource   string   `json:"viewsSource"`
	LikesCount    *int64   `json:"likesCount"`
	CommentsCount *int64   `json:"commentsCount"`
}

type ProviderResult struct {
	Platform        Platform
	Videos          []NormalizedVideo
	RawItemsCount   int
	SkippedNonVideo int
	Notes           []string
}

type RunRecord struct {
	ID                   int64      `json:"id"`
	Hashtag              string     `json:"hashtag"`
	Platforms            []Platform `json:"platforms"`
	LimitPerPlatform     int        `json:"limitPerPlatform"`
	Mode                 RunMode    `json:"mode"`
	Status               RunStatus  `json:"status"`
	RawItemsCount        int        `json:"rawItemsCount"`
	VideoItemsCount      int        `json:"videoItemsCount"`
	SkippedNonVideoCount int        `json:"skippedNonVideoCount"`
	MissingViewsCount    int        `json:"missingViewsCount"`
	TotalViewsSnapshot   int64      `json:"totalViewsSnapshot"`
	ErrorMessage         string     `json:"errorMessage,omitempty"`
	StartedAt            string     `json:"startedAt"`
	FinishedAt           string     `json:"finishedAt,omitempty"`
}

type PlatformBreakdown struct {
	Platform           Platform `json:"platform"`
	TotalViews         int64    `json:"totalViews"`
	TotalVideos        int      `json:"totalVideos"`
	VideosWithViews    int      `json:"videosWithViews"`
	VideosMissingViews int      `json:"videosMissingViews"`
}

type HashtagSummary struct {
	Hashtag            string                       `json:"hashtag"`
	GrandTotalViews    int64                        `json:"grandTotalViews"`
	TotalVideos        int                          `json:"totalVideos"`
	VideosWithViews    int                          `json:"videosWithViews"`
	VideosMissingViews int                          `json:"videosMissingViews"`
	DeltaViews         *int64                       `json:"deltaViews"`
	DeltaVideos        *int                         `json:"deltaVideos"`
	PreviousRunID      *int64                       `json:"previousRunId,omitempty"`
	LastUpdatedAt      string                       `json:"lastUpdatedAt"`
	Platforms          map[Platform]PlatformBreakdown `json:"platforms"`
	RunHistory         []RunRecord                  `json:"runHistory"`
	Disclaimer         string                       `json:"disclaimer"`
	IsAggregateOnly    bool                         `json:"isAggregateOnly,omitempty"`
}

type VideoViewRow struct {
	Platform      Platform `json:"platform"`
	Hashtag       string   `json:"hashtag"`
	VideoID       string   `json:"videoId"`
	Permalink     string   `json:"permalink"`
	AuthorName    string   `json:"authorName"`
	Caption       string   `json:"caption"`
	MediaType     string   `json:"mediaType"`
	PublishedAt   string   `json:"publishedAt"`
	ViewsCount    *int64   `json:"viewsCount"`
	ViewsSource   string   `json:"viewsSource"`
	LikesCount    *int64   `json:"likesCount"`
	CommentsCount *int64   `json:"commentsCount"`
	FirstSeenAt   string   `json:"firstSeenAt"`
	LastSeenAt    string   `json:"lastSeenAt"`
	LastFetchedAt string   `json:"lastFetchedAt"`
}

type TrackedHashtag struct {
	Hashtag            string `json:"hashtag"`
	GrandTotalViews    int64  `json:"grandTotalViews"`
	TotalVideos        int    `json:"totalVideos"`
	VideosMissingViews int    `json:"videosMissingViews"`
	InstagramViews     int64  `json:"instagramViews"`
	FacebookViews      int64  `json:"facebookViews"`
	TikTokViews        int64  `json:"tiktokViews"`
	LastRunAt          string `json:"lastRunAt"`
	LastRunStatus      string `json:"lastRunStatus"`
	LastRunMode        string `json:"lastRunMode"`
}

type VideoQueryOptions struct {
	Platform string
	Search   string
	Sort     string
	Limit    int
}
