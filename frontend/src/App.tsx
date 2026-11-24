import { useMemo, useState } from 'react';
import { AlertCircle, Loader2, X } from 'lucide-react';
import { ResultSummary } from './components/ResultSummary';
import { RunHistory } from './components/RunHistory';
import { SearchHeader, type ViewMode } from './components/SearchHeader';
import { SegmentedControl, type SegmentOption } from './components/SegmentedControl';
import { VideoList } from './components/VideoList';
import { useHashtagTracker } from './hooks/useHashtagTracker';
import { formatPlatformLabel } from './utils/format';

type DetailTab = 'videos' | 'history';

export function App() {
  const [viewMode, setViewMode] = useState<ViewMode>('simple');
  const [showSimpleVideos, setShowSimpleVideos] = useState(false);
  const [detailTab, setDetailTab] = useState<DetailTab>('videos');

  const {
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
  } = useHashtagTracker();

  const topVideoViews = useMemo(() => {
    let maxVal: number | null = null;
    for (const v of videos) {
      if (v.viewsCount !== null && (maxVal === null || v.viewsCount > maxVal)) {
        maxVal = v.viewsCount;
      }
    }
    return maxVal;
  }, [videos]);

  const detailTabOptions = useMemo<SegmentOption<DetailTab>[]>(() => {
    const totalVideos = summary?.totalVideos ?? 0;
    const totalRuns = summary?.runHistory.length ?? 0;
    return [
      {
        value: 'videos',
        label: (
          <>
            Daftar Video
            <span className="tabular-nums text-[11px] text-zinc-400">{totalVideos}</span>
          </>
        ),
      },
      {
        value: 'history',
        label: (
          <>
            Riwayat Pengecekan
            <span className="tabular-nums text-[11px] text-zinc-400">{totalRuns}</span>
          </>
        ),
      },
    ];
  }, [summary?.totalVideos, summary?.runHistory.length]);

  const platformLabel = formatPlatformLabel(activePlatform);
  const isDetailed = viewMode === 'detailed';

  return (
    <div className="relative min-h-screen overflow-x-clip bg-[#EEF2F7] px-4 py-10 text-zinc-950 sm:py-12">
      <div aria-hidden="true" className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
        <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.62)_0%,rgba(238,242,247,0.35)_50%,rgba(226,232,240,0.45)_100%)]" />

        <div
          className={`absolute inset-0 transition-opacity duration-700 ${
            !isBootstrapping && activePlatform === 'instagram' ? 'opacity-100' : 'opacity-0'
          }`}
        >
          <div className="absolute -top-28 left-[12%] h-[500px] w-[500px] rounded-full bg-[radial-gradient(circle,rgba(244,114,182,0.3)_0%,rgba(244,114,182,0)_70%)] blur-3xl" />
          <div className="absolute top-[16%] right-[10%] h-[480px] w-[480px] rounded-full bg-[radial-gradient(circle,rgba(251,146,60,0.26)_0%,rgba(251,146,60,0)_70%)] blur-3xl" />
          <div className="absolute bottom-[-110px] left-[34%] h-[460px] w-[460px] rounded-full bg-[radial-gradient(circle,rgba(167,139,250,0.26)_0%,rgba(167,139,250,0)_70%)] blur-3xl" />
        </div>

        <div
          className={`absolute inset-0 transition-opacity duration-700 ${
            !isBootstrapping && activePlatform === 'facebook' ? 'opacity-100' : 'opacity-0'
          }`}
        >
          <div className="absolute -top-28 left-[16%] h-[520px] w-[520px] rounded-full bg-[radial-gradient(circle,rgba(59,130,246,0.3)_0%,rgba(59,130,246,0)_70%)] blur-3xl" />
          <div className="absolute top-[18%] right-[12%] h-[460px] w-[460px] rounded-full bg-[radial-gradient(circle,rgba(56,189,248,0.26)_0%,rgba(56,189,248,0)_70%)] blur-3xl" />
          <div className="absolute bottom-[-100px] left-[30%] h-[440px] w-[440px] rounded-full bg-[radial-gradient(circle,rgba(99,102,241,0.22)_0%,rgba(99,102,241,0)_70%)] blur-3xl" />
        </div>

        <div
          className={`absolute inset-0 transition-opacity duration-700 ${
            !isBootstrapping && activePlatform === 'tiktok' ? 'opacity-100' : 'opacity-0'
          }`}
        >
          <div className="absolute -top-28 left-[12%] h-[500px] w-[500px] rounded-full bg-[radial-gradient(circle,rgba(37,244,238,0.3)_0%,rgba(37,244,238,0)_70%)] blur-3xl" />
          <div className="absolute top-[20%] right-[10%] h-[480px] w-[480px] rounded-full bg-[radial-gradient(circle,rgba(254,44,85,0.23)_0%,rgba(254,44,85,0)_70%)] blur-3xl" />
          <div className="absolute bottom-[-100px] left-[34%] h-[460px] w-[460px] rounded-full bg-[radial-gradient(circle,rgba(148,163,184,0.25)_0%,rgba(148,163,184,0)_70%)] blur-3xl" />
        </div>
      </div>

      {!isBootstrapping && (
        <main
          className={`ease-smooth relative z-10 mx-auto w-full space-y-6 transition-[max-width] duration-300 ${
            isDetailed ? 'max-w-5xl' : 'max-w-xl'
          }`}
        >
          <SearchHeader
            enabledPlatforms={enabledPlatforms}
            allowMockMode={allowMockMode}
            activePlatform={activePlatform}
          displayedPlatform={displayedPlatform}
          isPlatformFading={isPlatformFading}
          viewMode={viewMode}
          hasApifyToken={health?.hasApifyToken ?? false}
          isSubmitting={isSubmitting}
          selectedTag={selectedTag}
          trackedTags={trackedTags}
          rateLimit={activeRateLimit}
          cooldownSeconds={cooldownSeconds}
          resetSeconds={resetSeconds}
          onSelectPlatform={(p) => {
            setShowSimpleVideos(false);
            setDetailTab('videos');
            selectPlatform(p);
          }}
          onChangeViewMode={setViewMode}
          onSelectTag={(tag) => {
            setShowSimpleVideos(false);
            selectTag(tag);
          }}
          onSubmit={startRun}
          onFetchTikTok={fetchTikTok}
        />

        {activeRun && activeRun.status === 'running' && (
          <div
            aria-live="polite"
            className="animate-panel-in glass-surface flex items-center justify-center gap-2 rounded-2xl px-4 py-3 text-xs text-zinc-700"
          >
            <Loader2 className="h-3.5 w-3.5 animate-spin text-zinc-600" />
            <span>
              Menghitung total views <strong>#{activeRun.hashtag}</strong> di {platformLabel}...
            </span>
          </div>
        )}

        {errorMsg && (
          <div className="animate-panel-in glass-alert-error flex items-center justify-between gap-3 rounded-2xl px-4 py-3 text-xs text-rose-800">
            <div className="flex items-center gap-2">
              <AlertCircle className="h-4 w-4 shrink-0" />
              <span>{errorMsg}</span>
            </div>
            <button
              type="button"
              onClick={clearError}
              aria-label="Tutup pesan"
              className="rounded p-0.5 opacity-60 transition-opacity hover:opacity-100"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        )}

        <div
          className={`platform-fade-stage space-y-4 ${
            isPlatformFading ? 'platform-fade-out' : 'platform-fade-in'
          }`}
        >
          {summary && (
            <div key={viewMode} className="animate-fade-in space-y-4">
              <ResultSummary
                key={`${displayedPlatform}:${viewMode}`}
                activePlatform={displayedPlatform}
                viewMode={viewMode}
                summary={summary}
                topVideoViews={topVideoViews}
                showSimpleVideos={showSimpleVideos}
                onToggleSimpleVideos={() => setShowSimpleVideos((v) => !v)}
                isRefreshing={isSubmitting}
                isCoolingDown={cooldownSeconds > 0}
                onRefreshTikTok={(tag) => fetchTikTok(tag, true)}
              />

              {!isDetailed && !summary.isAggregateOnly && (
                <div
                  className={`ease-smooth grid transition-[grid-template-rows,opacity,margin] duration-300 ${
                    showSimpleVideos
                      ? 'mt-4 grid-rows-[1fr] opacity-100'
                      : 'pointer-events-none !mt-0 grid-rows-[0fr] opacity-0'
                  }`}
                >
                  <div className="overflow-hidden">
                    <VideoList
                      viewMode="simple"
                      videos={videos}
                      loading={videosLoading}
                      searchQuery={searchQuery}
                      sortBy={sortBy}
                      onSearchQueryChange={setSearchQuery}
                      onSortByChange={setSortBy}
                    />
                  </div>
                </div>
              )}

              {isDetailed && (
                <section className="glass-card overflow-hidden rounded-[28px]">
                  {summary.isAggregateOnly ? (
                    <>
                      <div className="border-b border-zinc-900/[0.06] px-6 py-3.5">
                        <span className="text-xs font-medium text-zinc-700">Riwayat Pengecekan</span>
                      </div>
                      <RunHistory runs={summary.runHistory} showMode={false} />
                    </>
                  ) : (
                    <>
                      <div className="flex items-center justify-between border-b border-zinc-900/[0.06] px-6 py-3">
                        <SegmentedControl
                          role="tablist"
                          ariaLabel="Detail hasil pengecekan"
                          value={detailTab}
                          options={detailTabOptions}
                          onChange={setDetailTab}
                          size="xs"
                        />
                      </div>

                      <div key={detailTab} className="animate-fade-in">
                        {detailTab === 'videos' ? (
                          <VideoList
                            viewMode="detailed"
                            videos={videos}
                            loading={videosLoading}
                            searchQuery={searchQuery}
                            sortBy={sortBy}
                            onSearchQueryChange={setSearchQuery}
                            onSortByChange={setSortBy}
                          />
                        ) : (
                          <RunHistory runs={summary.runHistory} />
                        )}
                      </div>
                    </>
                  )}
                </section>
              )}
            </div>
          )}
        </div>
      </main>
      )}
    </div>
  );
}

export default App;
