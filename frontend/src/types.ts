export type Platform = 'instagram' | 'facebook' | 'tiktok';
export type RunMode = 'live' | 'mock' | 'direct';
export type RunStatus = 'running' | 'completed' | 'partial' | 'failed';

export interface RateLimitStatus {
  platform: Platform;
  unlimited?: boolean;
  limit: number;
  remaining: number;
  used: number;
  windowSeconds: number;
  resetAt?: string;
  fullResetAt?: string;
  cooldownUntil?: string;
  retryAfterSeconds: number;
  upstreamBlocked?: boolean;
  note?: string;
}

export interface RateLimitsResponse {
  serverTime: string;
  limits: Record<Platform, RateLimitStatus>;
}

export interface HealthResponse {
  status: string;
  hasApifyToken: boolean;
  enabledPlatforms?: Platform[];
  allowMockMode?: boolean;
  metricFocus: string;
  databaseEngine: string;
  serverTime?: string;
  rateLimits?: Record<Platform, RateLimitStatus>;
}

export interface RunOptions {
  hashtag: string;
  platforms: Platform[];
  limit: number;
  mode: RunMode;
  enrichFb: boolean;
  igResultsType: 'reels' | 'posts';
}

export interface RunRecord {
  id: number;
  hashtag: string;
  platforms: Platform[];
  limitPerPlatform: number;
  mode: RunMode;
  status: RunStatus;
  rawItemsCount: number;
  videoItemsCount: number;
  skippedNonVideoCount: number;
  missingViewsCount: number;
  totalViewsSnapshot: number;
  errorMessage?: string;
  startedAt: string;
  finishedAt?: string;
}

export interface PlatformBreakdown {
  platform: Platform;
  totalViews: number;
  totalVideos: number;
  videosWithViews: number;
  videosMissingViews: number;
}

export interface HashtagSummary {
  hashtag: string;
  grandTotalViews: number;
  totalVideos: number;
  videosWithViews: number;
  videosMissingViews: number;
  deltaViews: number | null;
  deltaVideos: number | null;
  previousRunId?: number | null;
  lastUpdatedAt: string;
  platforms: Record<Platform, PlatformBreakdown>;
  runHistory: RunRecord[];
  disclaimer: string;
  isAggregateOnly?: boolean;
}

export interface TikTokFetchResponse {
  hashtag: string;
  summary: HashtagSummary;
  rateLimit?: RateLimitStatus;
  cached?: boolean;
  cacheExpiresAt?: string;
}

export interface VideoViewRow {
  platform: Platform;
  hashtag: string;
  videoId: string;
  permalink: string;
  authorName: string;
  caption: string;
  mediaType: string;
  publishedAt: string;
  viewsCount: number | null;
  viewsSource: string;
  likesCount: number | null;
  commentsCount: number | null;
  firstSeenAt: string;
  lastSeenAt: string;
  lastFetchedAt: string;
}

export interface TrackedHashtag {
  hashtag: string;
  grandTotalViews: number;
  totalVideos: number;
  videosMissingViews: number;
  instagramViews: number;
  facebookViews: number;
  tiktokViews: number;
  lastRunAt: string;
  lastRunStatus: string;
  lastRunMode: string;
}
