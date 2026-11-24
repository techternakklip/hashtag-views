import { ArrowUpRight, ChevronDown, Download, RefreshCw } from 'lucide-react';
import type { HashtagSummary, Platform } from '../types';
import { getExportUrl } from '../api/client';
import {
  formatCompact,
  formatCompactLong,
  formatDateTime,
  formatDelta,
  formatNumber,
  formatPlatformLabel,
  formatShortTime,
} from '../utils/format';
import { AnimatedNumber } from './AnimatedNumber';
import { PlatformIcon } from './PlatformIcon';
import type { ViewMode } from './SearchHeader';

interface ResultSummaryProps {
  activePlatform: Platform;
  viewMode: ViewMode;
  summary: HashtagSummary;
  topVideoViews: number | null;
  showSimpleVideos: boolean;
  onToggleSimpleVideos: () => void;
  isRefreshing?: boolean;
  isCoolingDown?: boolean;
  onRefreshTikTok?: (hashtag: string) => void;
}

export function ResultSummary({
  activePlatform,
  viewMode,
  summary,
  topVideoViews,
  showSimpleVideos,
  onToggleSimpleVideos,
  isRefreshing = false,
  isCoolingDown = false,
  onRefreshTikTok,
}: ResultSummaryProps) {
  const baseActionCls =
    'inline-flex h-8 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-full px-3.5 text-xs font-medium transition-all duration-200 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50';
  const platformLabel = formatPlatformLabel(activePlatform);
  const isAggregate = Boolean(summary.isAggregateOnly);
  const isHugeViews = summary.grandTotalViews >= 10_000_000_000;

  const actionButtons = (
    <div className="flex items-center gap-2">
      {isAggregate && onRefreshTikTok && (
        <button
          type="button"
          onClick={() => onRefreshTikTok(summary.hashtag)}
          disabled={isRefreshing || isCoolingDown}
          className={`glass-button-muted text-zinc-800 ${baseActionCls}`}
        >
          <RefreshCw
            className={`h-3.5 w-3.5 text-zinc-600 ${isRefreshing ? 'animate-spin' : ''}`}
          />
          Perbarui
        </button>
      )}
      <a
        href={getExportUrl(summary.hashtag, 'csv', activePlatform)}
        className={`glass-button-excel text-emerald-800 ${baseActionCls}`}
      >
        <Download className="h-3.5 w-3.5 text-emerald-700" />
        Unduh Excel
      </a>
    </div>
  );

  if (viewMode === 'simple') {
    return (
      <section className="glass-card overflow-hidden rounded-[28px]">
        <div className="px-6 py-10 text-center sm:px-8">
          <div className="glass-pill inline-flex items-center gap-2 rounded-full px-3.5 py-1 text-xs font-medium text-zinc-800">
            <PlatformIcon platform={activePlatform} className="h-4 w-4 shrink-0" />
            <span>
              #{summary.hashtag} · {platformLabel}
            </span>
          </div>

          <p className="mt-6 text-xs font-medium uppercase tracking-wider text-zinc-500">
            Total Views
          </p>
          <p className="mt-2 tabular-nums text-4xl font-bold leading-none tracking-[-0.03em] text-zinc-950 sm:text-5xl">
            <AnimatedNumber value={summary.grandTotalViews} />
          </p>

          <p className="mt-1.5 text-sm text-zinc-500">
            {formatCompactLong(summary.grandTotalViews)} views
          </p>

          <p className="mt-4 text-sm text-zinc-600">
            Dari{' '}
            <strong className="font-semibold text-zinc-950 tabular-nums">
              {formatNumber(summary.totalVideos)} video
            </strong>
            {isAggregate && (
              <span className="text-zinc-500">
                {' '}
                ({formatCompactLong(summary.totalVideos)} video)
              </span>
            )}
            {!isAggregate && summary.videosMissingViews > 0 && (
              <span className="text-zinc-500">
                {' '}
                ({summary.videosMissingViews} tidak menampilkan views)
              </span>
            )}
          </p>
        </div>

        <div className="flex min-h-[52px] flex-wrap items-center justify-between gap-2 px-6 pb-5 pt-1 text-xs">
          {isAggregate ? (
            <span className="text-zinc-500">
              Data per {formatDateTime(summary.lastUpdatedAt)}
            </span>
          ) : (
            <button
              type="button"
              onClick={onToggleSimpleVideos}
              aria-expanded={showSimpleVideos}
              className="inline-flex cursor-pointer items-center gap-1.5 font-medium text-zinc-700 transition-colors duration-200 hover:text-[#007AFF]"
            >
              <span>
                {showSimpleVideos
                  ? 'Sembunyikan rincian'
                  : `Lihat rincian (${summary.totalVideos} video)`}
              </span>
              <ChevronDown
                className={`ease-smooth h-3.5 w-3.5 transition-transform duration-300 ${
                  showSimpleVideos ? 'rotate-180' : ''
                }`}
              />
            </button>
          )}

          {actionButtons}
        </div>
      </section>
    );
  }

  const avgViews =
    summary.videosWithViews > 0
      ? Math.round(summary.grandTotalViews / summary.videosWithViews)
      : 0;

  const completedRuns = [...summary.runHistory]
    .filter((r) => r.status === 'completed' || r.status === 'partial')
    .slice(0, 8)
    .reverse();
  const maxSnapshotViews = Math.max(1, ...completedRuns.map((r) => r.totalViewsSnapshot));

  return (
    <section className="glass-card overflow-hidden rounded-[28px]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-900/[0.06] px-6 py-4">
        <div className="flex items-center gap-2.5">
          <PlatformIcon platform={activePlatform} className="h-5 w-5 shrink-0" />
          <div>
            <h2 className="text-lg font-semibold text-zinc-950">
              #{summary.hashtag}{' '}
              <span className="font-normal text-zinc-500">di {platformLabel}</span>
            </h2>
            <p className="text-xs text-zinc-500">
              Diperbarui {formatDateTime(summary.lastUpdatedAt)} · {summary.runHistory.length}{' '}
              pengecekan tersimpan
            </p>
          </div>
        </div>

        {actionButtons}
      </div>

      <div className="grid grid-cols-1 divide-y divide-zinc-900/[0.06] md:grid-cols-3 md:divide-x md:divide-y-0">
        <div className="p-6">
          <p className="text-xs font-medium text-zinc-500">Total Views</p>
          <p
            className={`mt-2 tabular-nums font-semibold tracking-tight text-zinc-950 ${
              isHugeViews ? 'text-2xl lg:text-[28px]' : 'text-3xl'
            }`}
          >
            <AnimatedNumber value={summary.grandTotalViews} />
          </p>
          <p className="mt-0.5 text-xs text-zinc-500">
            {formatCompactLong(summary.grandTotalViews)} views
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-1.5 text-xs">
            {summary.deltaViews !== null ? (
              <>
                <span className="inline-flex items-center gap-0.5 rounded-full border border-emerald-500/20 bg-emerald-500/12 px-2 py-0.5 tabular-nums font-medium text-emerald-700">
                  <ArrowUpRight className="h-3 w-3" />
                  {formatDelta(summary.deltaViews)}
                </span>
                <span className="text-zinc-500">dari pengecekan sebelumnya</span>
              </>
            ) : (
              <span className="text-zinc-500">Pengecekan pertama</span>
            )}
          </div>
        </div>

        <div className="p-6">
          <p className="text-xs font-medium text-zinc-500">
            {isAggregate ? 'Total Video' : 'Total Video Ditemukan'}
          </p>
          <p className="mt-2 tabular-nums text-3xl font-semibold tracking-tight text-zinc-950">
            <AnimatedNumber value={summary.totalVideos} durationMs={320} />
          </p>
          <p className="mt-2 tabular-nums text-xs text-zinc-600">
            {isAggregate ? (
              <>
                {formatCompactLong(summary.totalVideos)} video
                {summary.deltaVideos !== null && summary.deltaVideos > 0 && (
                  <span className="text-emerald-700">
                    {' '}
                    · +{formatNumber(summary.deltaVideos)} baru
                  </span>
                )}
              </>
            ) : (
              <>
                {formatNumber(summary.videosWithViews)} punya views
                {summary.videosMissingViews > 0 && (
                  <span className="text-amber-700">
                    {' '}
                    · {summary.videosMissingViews} disembunyikan
                  </span>
                )}
                {summary.deltaVideos !== null && summary.deltaVideos > 0 && (
                  <span className="text-emerald-700">
                    {' '}
                    · +{formatNumber(summary.deltaVideos)} baru
                  </span>
                )}
              </>
            )}
          </p>
        </div>

        <div className="p-6">
          <p className="text-xs font-medium text-zinc-500">Rata-rata Views / Video</p>
          <p className="mt-2 tabular-nums text-3xl font-semibold tracking-tight text-zinc-950">
            <AnimatedNumber value={avgViews} />
          </p>
          {!isAggregate && (
            <p className="mt-2 tabular-nums text-xs text-zinc-600">
              Video tertinggi:{' '}
              <span className="font-medium text-zinc-900">{formatNumber(topVideoViews)}</span>
            </p>
          )}
        </div>
      </div>

      <div
        className={
          isAggregate
            ? 'border-t border-zinc-900/[0.06]'
            : 'grid grid-cols-1 divide-y divide-zinc-900/[0.06] border-t border-zinc-900/[0.06] md:grid-cols-12 md:divide-x md:divide-y-0'
        }
      >
        <div className={isAggregate ? 'p-5' : 'p-5 md:col-span-6'}>
          <div className="flex items-center justify-between text-xs">
            <span className="font-medium text-zinc-800">Perkembangan Views per Pengecekan</span>
            <span className="text-[11px] text-zinc-500">{completedRuns.length} terakhir</span>
          </div>

          {completedRuns.length === 0 ? (
            <p className="mt-4 text-xs text-zinc-500">Belum ada riwayat selesai.</p>
          ) : (
            <div className="mt-3 flex h-20 items-end gap-3">
              {completedRuns.map((run) => {
                const pct = Math.max(
                  14,
                  Math.round((run.totalViewsSnapshot / maxSnapshotViews) * 100),
                );
                return (
                  <div
                    key={run.id}
                    className="group flex w-14 flex-col items-center gap-1"
                    title={`${formatDateTime(run.startedAt)}: ${formatNumber(run.totalViewsSnapshot)} views (${formatNumber(run.videoItemsCount)} video)`}
                  >
                    <span className="tabular-nums text-[10px] font-medium text-zinc-600 transition-colors group-hover:text-zinc-950">
                      {formatCompact(run.totalViewsSnapshot)}
                    </span>
                    <div className="flex h-11 w-8 items-end rounded-full bg-zinc-900/[0.06] p-0.5">
                      <div
                        style={{ height: `${pct}%` }}
                        className="ease-smooth w-full rounded-full bg-[#007AFF]/75 transition-[height,background-color] duration-500 group-hover:bg-[#007AFF]"
                      />
                    </div>
                    <span className="tabular-nums text-[10px] text-zinc-500">
                      {formatShortTime(run.startedAt)}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {!isAggregate && (
          <div className="glass-subbar flex items-center p-5 text-xs leading-relaxed text-zinc-600 md:col-span-6">
            <span>
              Angka di atas dihitung dari{' '}
              <strong className="font-medium text-zinc-800">total views asli</strong> (bukan reach)
              dari video {platformLabel} yang ditemukan pada hashtag ini.
            </span>
          </div>
        )}
      </div>
    </section>
  );
}
