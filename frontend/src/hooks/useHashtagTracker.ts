import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ApiError,
  fetchHashtagSummary,
  fetchHashtagVideos,
  fetchHealth,
  fetchRateLimits,
  fetchRunStatus,
  fetchTikTokHashtag,
  fetchTrackedHashtags,
  startHashtagRun,
} from '../api/client';
import type {
  HashtagSummary,
  HealthResponse,
  Platform,
  RateLimitStatus,
  RunOptions,
  RunRecord,
  TrackedHashtag,
  VideoViewRow,
} from '../types';

const PLATFORM_FADE_OUT_MS = 270;

function sleepWithSignal(ms: number, signal: AbortSignal): Promise<void> {
  if (ms <= 0) return Promise.resolve();
  return new Promise((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }
    const timer = window.setTimeout(() => {
      signal.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      window.clearTimeout(timer);
      resolve();
    };
    signal.addEventListener('abort', onAbort, { once: true });
  });
}

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

export function useHashtagTracker() {
  const [enabledPlatforms, setEnabledPlatforms] = useState<Platform[]>([
    'instagram',
    'facebook',
    'tiktok',
  ]);
  const [allowMockMode, setAllowMockMode] = useState(true);
  const [isBootstrapping, setIsBootstrapping] = useState(true);
  const [activePlatform, setActivePlatform] = useState<Platform>('instagram');
  const [displayedPlatform, setDisplayedPlatform] = useState<Platform>('instagram');
  const [isPlatformFading, setIsPlatformFading] = useState(false);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [trackedTags, setTrackedTags] = useState<TrackedHashtag[]>([]);
  const [selectedTag, setSelectedTag] = useState('');
  const [summary, setSummary] = useState<HashtagSummary | null>(null);
  const [videos, setVideos] = useState<VideoViewRow[]>([]);
  const [videosLoading, setVideosLoading] = useState(false);

  const [searchQuery, setSearchQuery] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [sortBy, setSortBy] = useState('views_desc');
  const [refreshTick, setRefreshTick] = useState(0);

  const [activeRun, setActiveRun] = useState<RunRecord | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');
  const [rateLimits, setRateLimits] = useState<Partial<Record<Platform, RateLimitStatus>>>({});
  const [nowMs, setNowMs] = useState(() => Date.now());

  const pollTimerRef = useRef<number | null>(null);
  const skipNextSummaryFetchRef = useRef<string | null>(null);
  const skipNextVideosFetchRef = useRef<string | null>(null);
  const selectedTagRef = useRef(selectedTag);
  selectedTagRef.current = selectedTag;
  const sortByRef = useRef(sortBy);
  sortByRef.current = sortBy;
  const expiredSyncKeyRef = useRef('');
  const switchAbortRef = useRef<AbortController | null>(null);
  const switchSeqRef = useRef(0);

  useEffect(() => {
    const t = window.setTimeout(() => setDebouncedSearch(searchQuery), 200);
    return () => window.clearTimeout(t);
  }, [searchQuery]);

  const refreshRateLimits = useCallback(async (signal?: AbortSignal) => {
    const res = await fetchRateLimits(signal);
    setRateLimits(res.limits);
    setNowMs(Date.now());
  }, []);

  const loadTrackedList = useCallback(
    async (platform: Platform, preferredTag?: string, signal?: AbortSignal) => {
      const tags = await fetchTrackedHashtags(platform, signal);
      setTrackedTags(tags);
      setSelectedTag((prev) => {
        if (preferredTag && tags.some((t) => t.hashtag === preferredTag)) {
          return preferredTag;
        }
        if (prev && tags.some((t) => t.hashtag === prev)) {
          return prev;
        }
        return tags.length > 0 ? tags[0].hashtag : '';
      });
    },
    [],
  );

  useEffect(() => {
    const ac = new AbortController();
    fetchHealth(ac.signal)
      .then((h) => {
        const enabled: Platform[] =
          h.enabledPlatforms && h.enabledPlatforms.length > 0
            ? h.enabledPlatforms
            : ['instagram', 'facebook', 'tiktok'];
        setHealth(h);
        setEnabledPlatforms(enabled);
        setAllowMockMode(h.allowMockMode ?? true);
        setActivePlatform(enabled[0]);
        setDisplayedPlatform(enabled[0]);
        if (h.rateLimits) {
          setRateLimits(h.rateLimits);
          setNowMs(Date.now());
        }
        return loadTrackedList(enabled[0], undefined, ac.signal);
      })
      .catch((err: unknown) => {
        if ((err as { name?: string })?.name === 'AbortError') return;
        setErrorMsg(err instanceof Error ? err.message : 'Gagal terhubung ke server');
      })
      .finally(() => {
        if (!ac.signal.aborted) {
          setIsBootstrapping(false);
        }
      });
    return () => ac.abort();
  }, [loadTrackedList]);

  const activeRateLimit = rateLimits[displayedPlatform];
  const cooldownUntilMs = activeRateLimit?.cooldownUntil
    ? Date.parse(activeRateLimit.cooldownUntil)
    : 0;
  const resetAtMs = activeRateLimit?.resetAt ? Date.parse(activeRateLimit.resetAt) : 0;
  const hasActiveCountdown = cooldownUntilMs > nowMs || resetAtMs > nowMs;

  useEffect(() => {
    if (!hasActiveCountdown) return;
    const id = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [hasActiveCountdown]);

  useEffect(() => {
    const targetMs = cooldownUntilMs > 0 ? cooldownUntilMs : resetAtMs;
    if (!targetMs || targetMs > nowMs) return;
    const key = `${displayedPlatform}:${targetMs}`;
    if (expiredSyncKeyRef.current === key) return;

    const ac = new AbortController();
    let retryTimer: number | null = null;
    let attempts = 0;

    const syncAfterExpiry = () => {
      attempts += 1;
      refreshRateLimits(ac.signal)
        .then(() => {
          if (!ac.signal.aborted) {
            expiredSyncKeyRef.current = key;
          }
        })
        .catch((err: unknown) => {
          if (ac.signal.aborted || (err as { name?: string })?.name === 'AbortError') return;
          if (attempts < 3) {
            retryTimer = window.setTimeout(syncAfterExpiry, attempts * 1500);
            return;
          }
          expiredSyncKeyRef.current = key;
          setErrorMsg(err instanceof Error ? err.message : 'Gagal memperbarui status kuota');
        });
    };

    syncAfterExpiry();
    return () => {
      ac.abort();
      if (retryTimer !== null) {
        window.clearTimeout(retryTimer);
      }
    };
  }, [displayedPlatform, cooldownUntilMs, resetAtMs, nowMs, refreshRateLimits]);

  useEffect(() => {
    if (!selectedTag) {
      setSummary(null);
      return;
    }
    const cacheKey = `${displayedPlatform}:${selectedTag}`;
    if (skipNextSummaryFetchRef.current === cacheKey) {
      skipNextSummaryFetchRef.current = null;
      return;
    }
    const ac = new AbortController();
    fetchHashtagSummary(selectedTag, displayedPlatform, ac.signal)
      .then(setSummary)
      .catch((err: unknown) => {
        if ((err as { name?: string })?.name === 'AbortError') return;
        setErrorMsg(err instanceof Error ? err.message : 'Gagal memuat ringkasan views');
      });
    return () => ac.abort();
  }, [selectedTag, displayedPlatform, refreshTick]);

  useEffect(() => {
    if (!selectedTag || displayedPlatform === 'tiktok') {
      setVideos([]);
      setVideosLoading(false);
      return;
    }
    const cacheKey = `${displayedPlatform}:${selectedTag}:${debouncedSearch}:${sortBy}`;
    if (skipNextVideosFetchRef.current === cacheKey) {
      skipNextVideosFetchRef.current = null;
      return;
    }
    const ac = new AbortController();
    setVideosLoading(true);
    fetchHashtagVideos(
      selectedTag,
      { platform: displayedPlatform, q: debouncedSearch, sort: sortBy, limit: 500 },
      ac.signal,
    )
      .then((rows) => {
        setVideos(rows);
        setVideosLoading(false);
      })
      .catch((err: unknown) => {
        if ((err as { name?: string })?.name === 'AbortError') return;
        setVideosLoading(false);
        setErrorMsg(err instanceof Error ? err.message : 'Gagal memuat daftar video');
      });
    return () => ac.abort();
  }, [selectedTag, displayedPlatform, debouncedSearch, sortBy, refreshTick]);

  useEffect(() => {
    return () => {
      if (pollTimerRef.current !== null) {
        window.clearTimeout(pollTimerRef.current);
      }
      switchAbortRef.current?.abort();
    };
  }, []);

  const pollRun = useCallback(
    (runId: number, tag: string, platform: Platform) => {
      const tick = async () => {
        try {
          const rec = await fetchRunStatus(runId);
          setActiveRun(rec);
          if (rec.status === 'running') {
            pollTimerRef.current = window.setTimeout(tick, 1200);
            return;
          }
          setIsSubmitting(false);
          if (rec.status !== 'failed') {
            await loadTrackedList(platform, tag);
            setSelectedTag(tag);
            setRefreshTick((n) => n + 1);
          } else {
            await loadTrackedList(platform);
            setErrorMsg(rec.errorMessage || 'Gagal menghitung views.');
          }
        } catch (err) {
          setIsSubmitting(false);
          setErrorMsg(err instanceof Error ? err.message : 'Gagal memantau proses');
        }
      };
      pollTimerRef.current = window.setTimeout(tick, 450);
    },
    [loadTrackedList],
  );

  const startRun = useCallback(
    async (opts: RunOptions) => {
      if (pollTimerRef.current !== null) {
        window.clearTimeout(pollTimerRef.current);
      }
      setErrorMsg('');
      setIsSubmitting(true);
      const targetPlatform = opts.platforms[0] ?? activePlatform;
      try {
        const rec = await startHashtagRun(opts);
        setActiveRun(rec);
        if (opts.mode === 'live') {
          void refreshRateLimits().catch((syncErr: unknown) => {
            setErrorMsg(
              syncErr instanceof Error ? syncErr.message : 'Gagal memperbarui status kuota',
            );
          });
        }
        pollRun(rec.id, rec.hashtag, targetPlatform);
      } catch (err) {
        setIsSubmitting(false);
        if (err instanceof ApiError && err.rateLimit) {
          setRateLimits((prev) => ({ ...prev, [err.rateLimit!.platform]: err.rateLimit! }));
          setNowMs(Date.now());
        }
        setErrorMsg(err instanceof Error ? err.message : 'Gagal memulai pencarian');
      }
    },
    [activePlatform, pollRun, refreshRateLimits],
  );

  const fetchTikTok = useCallback(async (hashtag: string, force = false) => {
    setErrorMsg('');
    setIsSubmitting(true);
    try {
      const res = await fetchTikTokHashtag(hashtag, force);
      if (res.rateLimit) {
        setRateLimits((prev) => ({ ...prev, tiktok: res.rateLimit! }));
        setNowMs(Date.now());
      }
      if (selectedTagRef.current !== res.hashtag) {
        skipNextSummaryFetchRef.current = `tiktok:${res.hashtag}`;
      }
      setDisplayedPlatform('tiktok');
      setIsPlatformFading(false);
      setSummary(res.summary);
      setSelectedTag(res.hashtag);
      const tags = await fetchTrackedHashtags('tiktok');
      setTrackedTags(tags);
    } catch (err) {
      if (err instanceof ApiError && err.rateLimit) {
        setRateLimits((prev) => ({ ...prev, tiktok: err.rateLimit! }));
        setNowMs(Date.now());
      }
      setErrorMsg(err instanceof Error ? err.message : 'Gagal mengambil data TikTok');
    } finally {
      setIsSubmitting(false);
    }
  }, []);

  const selectPlatform = useCallback(
    (platform: Platform) => {
      if (platform === activePlatform || !enabledPlatforms.includes(platform)) return;

      switchAbortRef.current?.abort();
      const ac = new AbortController();
      switchAbortRef.current = ac;
      const seq = ++switchSeqRef.current;

      skipNextSummaryFetchRef.current = null;
      skipNextVideosFetchRef.current = null;
      setNowMs(Date.now());
      setErrorMsg('');
      setSearchQuery('');
      setDebouncedSearch('');
      setActiveRun((prev) => (prev && prev.status !== 'running' ? null : prev));
      setActivePlatform(platform);
      setIsPlatformFading(true);

      const fadeWait = sleepWithSignal(prefersReducedMotion() ? 0 : PLATFORM_FADE_OUT_MS, ac.signal);

      const loadTarget = async () => {
        const tags = await fetchTrackedHashtags(platform, ac.signal);
        const nextTag = tags.length > 0 ? tags[0].hashtag : '';
        if (!nextTag) {
          return { tags, nextTag: '', nextSummary: null as HashtagSummary | null, nextVideos: [] as VideoViewRow[] };
        }
        const [nextSummary, nextVideos] = await Promise.all([
          fetchHashtagSummary(nextTag, platform, ac.signal),
          platform === 'tiktok'
            ? Promise.resolve<VideoViewRow[]>([])
            : fetchHashtagVideos(
                nextTag,
                { platform, q: '', sort: sortByRef.current, limit: 500 },
                ac.signal,
              ),
        ]);
        return { tags, nextTag, nextSummary, nextVideos };
      };

      void Promise.all([fadeWait, loadTarget()])
        .then(([, data]) => {
          if (ac.signal.aborted || seq !== switchSeqRef.current) return;
          if (data.nextTag) {
            skipNextSummaryFetchRef.current = `${platform}:${data.nextTag}`;
            if (platform !== 'tiktok') {
              skipNextVideosFetchRef.current = `${platform}:${data.nextTag}::${sortByRef.current}`;
            }
          }
          setDisplayedPlatform(platform);
          setTrackedTags(data.tags);
          setSelectedTag(data.nextTag);
          setSummary(data.nextSummary);
          setVideos(data.nextVideos);
          setVideosLoading(false);

          window.requestAnimationFrame(() => {
            window.requestAnimationFrame(() => {
              if (seq === switchSeqRef.current) {
                setIsPlatformFading(false);
              }
            });
          });
        })
        .catch((err: unknown) => {
          if (ac.signal.aborted || seq !== switchSeqRef.current) return;
          if ((err as { name?: string })?.name === 'AbortError') return;
          setDisplayedPlatform(platform);
          setTrackedTags([]);
          setSelectedTag('');
          setSummary(null);
          setVideos([]);
          setVideosLoading(false);
          setIsPlatformFading(false);
          setErrorMsg(err instanceof Error ? err.message : 'Gagal memuat data platform');
        });
    },
    [activePlatform, enabledPlatforms],
  );

  const selectTag = useCallback((tag: string) => {
    skipNextSummaryFetchRef.current = null;
    skipNextVideosFetchRef.current = null;
    setErrorMsg('');
    setActiveRun((prev) => (prev && prev.status !== 'running' ? null : prev));
    setSelectedTag(tag);
  }, []);

  const clearError = useCallback(() => setErrorMsg(''), []);

  const cooldownSeconds =
    cooldownUntilMs > nowMs ? Math.ceil((cooldownUntilMs - nowMs) / 1000) : 0;
  const resetSeconds = resetAtMs > nowMs ? Math.ceil((resetAtMs - nowMs) / 1000) : 0;

  return {
    enabledPlatforms,
    allowMockMode,
    isBootstrapping,
    activePlatform,
    displayedPlatform,
    isPlatformFading,
    selectPlatform,
    health,
    trackedTags,
    selectedTag,
    summary,
    videos,
    videosLoading,
    searchQuery,
    setSearchQuery,
    sortBy,
    setSortBy,
    activeRun,
    isSubmitting,
    errorMsg,
    activeRateLimit,
    cooldownSeconds,
    resetSeconds,
    startRun,
    fetchTikTok,
    selectTag,
    clearError,
  };
}
