import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { Clock, Loader2, Search, SlidersHorizontal } from 'lucide-react';
import type { Platform, RateLimitStatus, RunMode, RunOptions, TrackedHashtag } from '../types';
import { formatCompact, formatDurationClock, formatPlatformLabel } from '../utils/format';
import { PlatformIcon } from './PlatformIcon';
import { RateLimitBar } from './RateLimitBar';
import { SegmentedControl, type SegmentOption } from './SegmentedControl';
import { SelectMenu, type SelectOption } from './SelectMenu';

export type ViewMode = 'simple' | 'detailed';

interface SearchHeaderProps {
  enabledPlatforms: Platform[];
  allowMockMode: boolean;
  activePlatform: Platform;
  displayedPlatform: Platform;
  isPlatformFading: boolean;
  viewMode: ViewMode;
  hasApifyToken: boolean;
  isSubmitting: boolean;
  selectedTag: string;
  trackedTags: TrackedHashtag[];
  rateLimit?: RateLimitStatus;
  cooldownSeconds: number;
  resetSeconds: number;
  onSelectPlatform: (platform: Platform) => void;
  onChangeViewMode: (mode: ViewMode) => void;
  onSelectTag: (tag: string) => void;
  onSubmit: (opts: RunOptions) => void;
  onFetchTikTok: (hashtag: string, force?: boolean) => void;
}

const PLATFORM_OPTIONS: SegmentOption<Platform>[] = [
  {
    value: 'instagram',
    label: (
      <>
        <PlatformIcon platform="instagram" className="h-4 w-4 shrink-0" />
        Instagram
      </>
    ),
  },
  {
    value: 'facebook',
    label: (
      <>
        <PlatformIcon platform="facebook" className="h-4 w-4 shrink-0" />
        Facebook
      </>
    ),
  },
  {
    value: 'tiktok',
    label: (
      <>
        <PlatformIcon platform="tiktok" className="h-4 w-4 shrink-0" />
        TikTok
      </>
    ),
  },
];

const VIEW_MODE_OPTIONS: SegmentOption<ViewMode>[] = [
  { value: 'simple', label: 'Ringkas' },
  { value: 'detailed', label: 'Lengkap' },
];

const LIMIT_OPTIONS: SelectOption<number>[] = [
  { value: 20, label: '20 video' },
  { value: 50, label: '50 video' },
  { value: 100, label: '100 video' },
  { value: 250, label: '250 video' },
  { value: 500, label: '500 video' },
];

const IG_FOCUS_OPTIONS: SelectOption<'reels' | 'posts'>[] = [
  { value: 'reels', label: 'Reels saja' },
  { value: 'posts', label: 'Semua post (filter video)' },
];

export function SearchHeader({
  enabledPlatforms,
  allowMockMode,
  activePlatform,
  displayedPlatform,
  isPlatformFading,
  viewMode,
  hasApifyToken,
  isSubmitting,
  selectedTag,
  trackedTags,
  rateLimit,
  cooldownSeconds,
  resetSeconds,
  onSelectPlatform,
  onChangeViewMode,
  onSelectTag,
  onSubmit,
  onFetchTikTok,
}: SearchHeaderProps) {
  const [hashtag, setHashtag] = useState(selectedTag || '');
  const [limit, setLimit] = useState(50);
  const [mode, setMode] = useState<RunMode>(!allowMockMode || hasApifyToken ? 'live' : 'mock');
  const [enrichFb, setEnrichFb] = useState(false);
  const [igResultsType, setIgResultsType] = useState<'reels' | 'posts'>('reels');
  const [showOptions, setShowOptions] = useState(false);
  const [optionsOverflow, setOptionsOverflow] = useState(false);
  const [localError, setLocalError] = useState('');
  const railRef = useRef<HTMLDivElement | null>(null);
  const [railFade, setRailFade] = useState({ left: false, right: false });

  const updateRailFade = useCallback(() => {
    const el = railRef.current;
    if (!el) {
      setRailFade((prev) => (prev.left || prev.right ? { left: false, right: false } : prev));
      return;
    }
    const left = el.scrollLeft > 2;
    const right = el.scrollLeft + el.clientWidth < el.scrollWidth - 2;
    setRailFade((prev) => (prev.left === left && prev.right === right ? prev : { left, right }));
  }, []);

  useEffect(() => {
    const frame = requestAnimationFrame(updateRailFade);
    const el = railRef.current;
    if (!el || typeof ResizeObserver === 'undefined') {
      return () => cancelAnimationFrame(frame);
    }
    const observer = new ResizeObserver(() => updateRailFade());
    observer.observe(el);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [trackedTags, displayedPlatform, viewMode, updateRailFade]);

  useEffect(() => {
    if (!allowMockMode) {
      setMode('live');
      return;
    }
    setMode((prev) => {
      if (!hasApifyToken && prev === 'live') return 'mock';
      return prev;
    });
  }, [hasApifyToken, allowMockMode]);

  useEffect(() => {
    setLocalError('');
    setShowOptions(false);
  }, [activePlatform]);

  useEffect(() => {
    setHashtag(selectedTag || '');
  }, [selectedTag, displayedPlatform]);

  const isInstagram = displayedPlatform === 'instagram';
  const isFacebook = displayedPlatform === 'facebook';
  const isTikTok = displayedPlatform === 'tiktok';
  const platformLabel = formatPlatformLabel(displayedPlatform);
  const isDetailed = viewMode === 'detailed';

  useEffect(() => {
    if (!(isDetailed && showOptions && !isTikTok)) {
      setOptionsOverflow(false);
      return;
    }
    const timer = window.setTimeout(() => setOptionsOverflow(true), 300);
    return () => window.clearTimeout(timer);
  }, [isDetailed, showOptions, isTikTok]);
  const isSimulated = !isTikTok && allowMockMode && (!hasApifyToken || mode === 'mock');
  const isCoolingDown = !isSimulated && cooldownSeconds > 0;
  const visiblePlatformOptions = useMemo(() => {
    const byValue = new Map(PLATFORM_OPTIONS.map((opt) => [opt.value, opt]));
    return enabledPlatforms
      .map((p) => byValue.get(p))
      .filter((opt): opt is SegmentOption<Platform> => Boolean(opt));
  }, [enabledPlatforms]);
  const railMaskCls =
    railFade.left && railFade.right
      ? 'history-mask-both'
      : railFade.left
        ? 'history-mask-left'
        : railFade.right
          ? 'history-mask-right'
          : '';
  const fadeStageCls = `platform-fade-stage ${
    isPlatformFading ? 'platform-fade-out' : 'platform-fade-in'
  }`;

  const sourceOptions = useMemo<SegmentOption<RunMode>[]>(() => {
    const liveOption: SegmentOption<RunMode> = {
      value: 'live',
      label: 'Live',
      disabled: !hasApifyToken,
      title: hasApifyToken
        ? 'Tarik data nyata via Apify'
        : 'Isi APIFY_API_TOKEN di .env untuk mengaktifkan data nyata',
    };
    return allowMockMode ? [{ value: 'mock', label: 'Simulasi' }, liveOption] : [liveOption];
  }, [hasApifyToken, allowMockMode]);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    setLocalError('');
    const cleaned = hashtag.trim().replace(/^#+/, '').replace(/\s+/g, '');
    if (!cleaned) {
      setLocalError('Ketik hashtag terlebih dahulu.');
      return;
    }

    if (activePlatform === 'tiktok') {
      onFetchTikTok(cleaned, false);
      return;
    }

    onSubmit({
      hashtag: cleaned,
      platforms: [activePlatform],
      limit: isDetailed ? limit : 50,
      mode: allowMockMode ? (hasApifyToken ? mode : 'mock') : 'live',
      enrichFb: activePlatform === 'facebook' && isDetailed && enrichFb,
      igResultsType: isDetailed ? igResultsType : 'reels',
    });
  };

  return (
    <div className="relative z-20 space-y-5">
      <div
        className={`flex flex-wrap items-center gap-3 ${
          visiblePlatformOptions.length > 1
            ? 'justify-between'
            : isDetailed
              ? 'justify-end'
              : 'justify-center'
        }`}
      >
        {visiblePlatformOptions.length > 1 && (
          <SegmentedControl
            role="tablist"
            ariaLabel="Pilih platform"
            value={activePlatform}
            options={visiblePlatformOptions}
            onChange={onSelectPlatform}
            size="md"
          />
        )}

        <SegmentedControl
          ariaLabel="Mode tampilan"
          value={viewMode}
          options={VIEW_MODE_OPTIONS}
          onChange={onChangeViewMode}
          size="sm"
        />
      </div>

      <div className={isDetailed ? 'text-left' : 'text-center'}>
        <h1 className="text-xl font-semibold tracking-[-0.02em] text-zinc-950">
          Cek Total Views Hashtag
        </h1>
      </div>

      <form onSubmit={handleSubmit}>
        <div className="relative z-30 flex flex-col gap-2 sm:flex-row">
          <div className="relative flex-1">
            <label htmlFor="hashtag-input" className="sr-only">
              Ketik hashtag {platformLabel}
            </label>
            <span className="pointer-events-none absolute inset-y-0 left-4 z-10 flex items-center text-base font-medium text-zinc-400">
              #
            </span>
            <input
              id="hashtag-input"
              type="text"
              autoComplete="off"
              spellCheck={false}
              value={hashtag}
              onChange={(e) => setHashtag(e.target.value)}
              placeholder={`Masukkan hashtag ${platformLabel} (contoh: kuliner, skincare)`}
              className="glass-surface h-11 w-full rounded-full pl-9 pr-5 text-sm text-zinc-950 placeholder-zinc-400 transition-all duration-200 focus:border-[#007AFF]/45 focus:bg-white/75 focus:outline-none focus:ring-2 focus:ring-[#007AFF]/18"
            />
          </div>

          <div className="flex items-center gap-2">
            {isDetailed && !isTikTok && (
              <div className="animate-panel-in flex items-center gap-2">
                <SelectMenu
                  ariaLabel="Jumlah video yang dicek"
                  value={limit}
                  options={LIMIT_OPTIONS}
                  onChange={setLimit}
                  size="lg"
                />

                <button
                  type="button"
                  onClick={() => setShowOptions((v) => !v)}
                  aria-expanded={showOptions}
                  className={`inline-flex h-11 items-center gap-1.5 rounded-full px-4 text-xs font-medium transition-all duration-200 active:scale-[0.98] ${
                    showOptions
                      ? 'glass-pill text-zinc-950'
                      : 'glass-surface text-zinc-600 hover:bg-white/75 hover:text-zinc-950'
                  }`}
                >
                  <SlidersHorizontal className="h-3.5 w-3.5" />
                  Opsi
                </button>
              </div>
            )}

            <button
              type="submit"
              disabled={isSubmitting || isCoolingDown}
              className="glass-button-primary inline-flex h-11 flex-1 items-center justify-center gap-2 rounded-full px-6 text-sm font-medium text-white transition-all duration-200 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-55 sm:flex-none"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Mencari...
                </>
              ) : isCoolingDown ? (
                <>
                  <Clock className="h-4 w-4" />
                  <span className="tabular-nums">Tunggu {formatDurationClock(cooldownSeconds)}</span>
                </>
              ) : (
                <>
                  <Search className="h-4 w-4" />
                  Cek Views
                </>
              )}
            </button>
          </div>
        </div>

        <div className={fadeStageCls}>
          <RateLimitBar
            platform={displayedPlatform}
            status={rateLimit}
            isSimulated={isSimulated}
            isDetailed={isDetailed}
            cooldownSec={cooldownSeconds}
            resetSec={resetSeconds}
          />
        </div>

        {!isTikTok && (
          <div
            className={`ease-smooth relative z-20 grid transition-[grid-template-rows,opacity,margin] duration-300 ${
              isDetailed && showOptions
                ? 'mt-2.5 grid-rows-[1fr] opacity-100'
                : 'pointer-events-none mt-0 grid-rows-[0fr] opacity-0'
            }`}
          >
            <div className={optionsOverflow ? 'overflow-visible' : 'overflow-hidden'}>
              <div className="glass-card flex flex-wrap items-center gap-x-6 gap-y-2.5 rounded-3xl px-5 py-3.5 text-xs text-zinc-700">
                {allowMockMode ? (
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-zinc-500">Sumber Data:</span>
                    <SegmentedControl
                      ariaLabel="Sumber Data"
                      value={mode}
                      options={sourceOptions}
                      onChange={setMode}
                      size="xs"
                    />
                    {!hasApifyToken && (
                      <span className="text-[11px] text-zinc-400">(Token belum dipasang)</span>
                    )}
                  </div>
                ) : (
                  !hasApifyToken && (
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-zinc-500">Sumber Data:</span>
                      <span className="text-[11px] text-amber-700">
                        Token Apify belum terpasang di server
                      </span>
                    </div>
                  )
                )}

                {isInstagram && (
                  <div className="inline-flex items-center gap-2">
                    <span className="font-medium text-zinc-500">Fokus Instagram:</span>
                    <SelectMenu
                      ariaLabel="Fokus konten Instagram"
                      value={igResultsType}
                      options={IG_FOCUS_OPTIONS}
                      onChange={setIgResultsType}
                      size="sm"
                    />
                  </div>
                )}

                {isFacebook && (
                  <label className="inline-flex cursor-pointer items-center gap-2">
                    <input
                      type="checkbox"
                      checked={enrichFb}
                      onChange={(e) => setEnrichFb(e.target.checked)}
                      className="h-3.5 w-3.5 rounded border-zinc-300 text-[#007AFF] accent-[#007AFF]"
                    />
                    <span>
                      Cek detail Reels Facebook{' '}
                      <span className="text-zinc-400">(jika views disembunyikan)</span>
                    </span>
                  </label>
                )}
              </div>
            </div>
          </div>
        )}

        {localError && (
          <p className="animate-panel-in mt-2 text-xs font-medium text-rose-600">{localError}</p>
        )}
      </form>

      {trackedTags.length > 0 && (
        <div
          className={`flex w-full items-center gap-2 ${
            isDetailed ? 'justify-start' : 'justify-center'
          } ${fadeStageCls}`}
        >
          <span className="shrink-0 whitespace-nowrap text-xs text-zinc-500">
            Riwayat {platformLabel}:
          </span>
          <div
            ref={railRef}
            onScroll={updateRailFade}
            className={`no-scrollbar -my-2 flex min-w-0 flex-nowrap items-center gap-1.5 overflow-x-auto px-0.5 py-2 ${railMaskCls}`}
          >
            {trackedTags.map((item) => {
              const active = item.hashtag === selectedTag;
              return (
                <button
                  key={item.hashtag}
                  type="button"
                  onClick={() => onSelectTag(item.hashtag)}
                  className={`inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-3 py-1 text-xs transition-all duration-200 active:scale-[0.97] ${
                    active
                      ? 'glass-chip-active font-medium text-white'
                      : 'glass-surface text-zinc-700 hover:bg-white/75 hover:text-zinc-950'
                  }`}
                >
                  <span>#{item.hashtag}</span>
                  {isDetailed && (
                    <span className={`tabular-nums ${active ? 'text-white/80' : 'text-zinc-500'}`}>
                      {formatCompact(item.grandTotalViews)}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
