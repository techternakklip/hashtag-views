package provider

import (
	"context"

	"meta-hashtag-views/internal/domain"
)

type HashtagProvider interface {
	Platform() domain.Platform
	FetchVideos(ctx context.Context, opts domain.RunOptions) (domain.ProviderResult, error)
}

type ActorRunner interface {
	RunActorAndFetchDataset(ctx context.Context, actorID string, input any) ([]map[string]any, error)
}
