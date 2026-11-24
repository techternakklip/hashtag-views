import type {
  HashtagSummary,
  HealthResponse,
  Platform,
  RateLimitsResponse,
  RateLimitStatus,
  RunOptions,
  RunRecord,
  TikTokFetchResponse,
  TrackedHashtag,
  VideoViewRow,
} from '../types';

const API_BASE = import.meta.env.VITE_API_BASE ?? '';

export class ApiError extends Error {
  status: number;
  rateLimit?: RateLimitStatus;

  constructor(message: string, status: number, rateLimit?: RateLimitStatus) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.rateLimit = rateLimit;
  }
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, init);
  const text = await res.text();
  let parsed: unknown = null;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      throw new ApiError(`Respons server tidak valid (${res.status})`, res.status);
    }
  }
  if (!res.ok) {
    const obj = typeof parsed === 'object' && parsed !== null ? (parsed as Record<string, unknown>) : null;
    const errMsg = obj && 'error' in obj ? String(obj.error) : `HTTP ${res.status}`;
    const rl = obj && 'rateLimit' in obj ? (obj.rateLimit as RateLimitStatus) : undefined;
    throw new ApiError(errMsg, res.status, rl);
  }
  return parsed as T;
}

export function fetchHealth(signal?: AbortSignal): Promise<HealthResponse> {
  return requestJSON<HealthResponse>('/api/health', { signal });
}

export function fetchRateLimits(signal?: AbortSignal): Promise<RateLimitsResponse> {
  return requestJSON<RateLimitsResponse>('/api/rate-limits', { signal });
}

export async function fetchTrackedHashtags(
  platform: Platform,
  signal?: AbortSignal,
): Promise<TrackedHashtag[]> {
  const data = await requestJSON<{ hashtags: TrackedHashtag[] }>(
    `/api/hashtags?platform=${encodeURIComponent(platform)}`,
    { signal },
  );
  return data.hashtags;
}

export function fetchHashtagSummary(
  tag: string,
  platform: Platform,
  signal?: AbortSignal,
): Promise<HashtagSummary> {
  return requestJSON<HashtagSummary>(
    `/api/hashtags/${encodeURIComponent(tag)}/summary?platform=${encodeURIComponent(platform)}`,
    { signal },
  );
}

export async function fetchHashtagVideos(
  tag: string,
  params: { platform: Platform; q?: string; sort?: string; limit?: number },
  signal?: AbortSignal,
): Promise<VideoViewRow[]> {
  const sp = new URLSearchParams({ platform: params.platform });
  if (params.q) sp.set('q', params.q);
  if (params.sort) sp.set('sort', params.sort);
  if (params.limit) sp.set('limit', String(params.limit));
  const data = await requestJSON<{ videos: VideoViewRow[] }>(
    `/api/hashtags/${encodeURIComponent(tag)}/videos?${sp.toString()}`,
    { signal },
  );
  return data.videos;
}

export function startHashtagRun(opts: RunOptions): Promise<RunRecord> {
  return requestJSON<RunRecord>('/api/runs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  });
}

export function fetchRunStatus(runId: number, signal?: AbortSignal): Promise<RunRecord> {
  return requestJSON<RunRecord>(`/api/runs/${runId}`, { signal });
}

export function fetchTikTokHashtag(hashtag: string, force = false): Promise<TikTokFetchResponse> {
  return requestJSON<TikTokFetchResponse>('/api/tiktok/fetch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(force ? { hashtag, force: true } : { hashtag }),
  });
}

export function getExportUrl(tag: string, format: 'csv' | 'json', platform: Platform): string {
  const sp = new URLSearchParams({ format, platform });
  return `${API_BASE}/api/hashtags/${encodeURIComponent(tag)}/export?${sp.toString()}`;
}
