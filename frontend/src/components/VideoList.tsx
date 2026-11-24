import { ExternalLink, Search } from 'lucide-react';
import type { VideoViewRow } from '../types';
import { formatNumber, formatShortDate } from '../utils/format';
import type { ViewMode } from './SearchHeader';
import { SelectMenu, type SelectOption } from './SelectMenu';

const SORT_OPTIONS: SelectOption<string>[] = [
  { value: 'views_desc', label: 'Views terbanyak' },
  { value: 'views_asc', label: 'Views paling sedikit' },
  { value: 'published_desc', label: 'Upload terbaru' },
];

interface VideoListProps {
  viewMode: ViewMode;
  videos: VideoViewRow[];
  loading: boolean;
  searchQuery: string;
  sortBy: string;
  onSearchQueryChange: (q: string) => void;
  onSortByChange: (sort: string) => void;
}

export function VideoList({
  viewMode,
  videos,
  loading,
  searchQuery,
  sortBy,
  onSearchQueryChange,
  onSortByChange,
}: VideoListProps) {
  if (viewMode === 'simple') {
    if (loading) {
      return (
        <div className="glass-card rounded-[28px] py-10 text-center text-xs text-zinc-500">
          Memuat rincian video...
        </div>
      );
    }
    if (videos.length === 0) return null;

    return (
      <section className="glass-card overflow-hidden rounded-[28px]">
        <ul className="divide-y divide-zinc-900/[0.06]">
          {videos.map((v) => (
            <li
              key={`${v.platform}-${v.videoId}`}
              className="flex items-center justify-between gap-4 px-5 py-3 text-sm transition-colors hover:bg-white/45"
            >
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium text-zinc-900">
                  {v.authorName || v.videoId}
                </p>
                <p className="truncate text-xs text-zinc-600">
                  {v.caption || '(tanpa keterangan)'}
                </p>
              </div>

              <div className="flex shrink-0 items-center gap-3">
                {v.viewsCount !== null ? (
                  <span className="tabular-nums text-sm font-semibold text-zinc-950">
                    {formatNumber(v.viewsCount)}{' '}
                    <span className="text-xs font-normal text-zinc-500">views</span>
                  </span>
                ) : (
                  <span className="text-xs text-amber-700">Tidak ditampilkan</span>
                )}

                {v.permalink && (
                  <a
                    href={v.permalink}
                    target="_blank"
                    rel="noreferrer"
                    aria-label={`Buka video ${v.authorName || v.videoId}`}
                    className="rounded-full p-1.5 text-zinc-500 transition hover:bg-white/70 hover:text-zinc-950"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                  </a>
                )}
              </div>
            </li>
          ))}
        </ul>
      </section>
    );
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-900/[0.06] px-6 py-3.5">
        <div className="relative flex-1 sm:max-w-xs">
          <Search className="pointer-events-none absolute left-2.5 top-2 z-10 h-3.5 w-3.5 text-zinc-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchQueryChange(e.target.value)}
            placeholder="Cari akun atau keterangan..."
            className="glass-surface h-8 w-full rounded-full pl-8 pr-3.5 text-xs text-zinc-900 placeholder-zinc-400 transition focus:border-[#007AFF]/45 focus:bg-white/75 focus:outline-none focus:ring-2 focus:ring-[#007AFF]/15"
          />
        </div>

        <SelectMenu
          ariaLabel="Urutkan video"
          value={sortBy}
          options={SORT_OPTIONS}
          onChange={onSortByChange}
          size="sm"
          align="right"
        />
      </div>

      {loading ? (
        <div className="py-12 text-center text-xs text-zinc-500">Memuat daftar video...</div>
      ) : videos.length === 0 ? (
        <div className="py-12 text-center text-xs text-zinc-500">
          Tidak ada video yang cocok dengan pencarian.
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="glass-subbar border-b border-zinc-900/[0.06] text-[11px] font-medium uppercase tracking-wider text-zinc-500">
                <th className="w-10 py-2.5 pl-6 pr-2">#</th>
                <th className="py-2.5 pr-4">Tipe</th>
                <th className="py-2.5 pr-4">Akun & Keterangan</th>
                <th className="py-2.5 pr-4 text-right">Views</th>
                <th className="py-2.5 pr-4 text-right">Likes</th>
                <th className="py-2.5 pr-4">Tanggal</th>
                <th className="py-2.5 pl-2 pr-6 text-right">Link</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-900/[0.05]">
              {videos.map((v, idx) => (
                <tr key={`${v.platform}-${v.videoId}`} className="transition-colors hover:bg-white/45">
                  <td className="py-3 pl-6 pr-2 tabular-nums text-zinc-500">{idx + 1}</td>
                  <td className="py-3 pr-4 whitespace-nowrap">
                    <span className="rounded-full bg-zinc-900/[0.06] px-2 py-0.5 text-[10px] font-medium uppercase text-zinc-700">
                      {v.mediaType}
                    </span>
                  </td>
                  <td className="max-w-md py-3 pr-4">
                    <p className="truncate font-medium text-zinc-900">
                      {v.authorName || v.videoId}
                    </p>
                    <p className="mt-0.5 truncate text-zinc-600">
                      {v.caption || '(tanpa keterangan)'}
                    </p>
                  </td>
                  <td className="py-3 pr-4 text-right whitespace-nowrap">
                    {v.viewsCount !== null ? (
                      <span className="tabular-nums font-semibold text-zinc-950">
                        {formatNumber(v.viewsCount)}
                      </span>
                    ) : (
                      <span className="inline-flex rounded-full border border-amber-500/20 bg-amber-500/12 px-2 py-0.5 text-[11px] font-medium text-amber-700">
                        Tidak ditampilkan
                      </span>
                    )}
                  </td>
                  <td className="py-3 pr-4 text-right tabular-nums text-zinc-700">
                    {formatNumber(v.likesCount)}
                  </td>
                  <td className="py-3 pr-4 whitespace-nowrap tabular-nums text-zinc-600">
                    {formatShortDate(v.publishedAt)}
                  </td>
                  <td className="py-3 pl-2 pr-6 text-right whitespace-nowrap">
                    {v.permalink ? (
                      <a
                        href={v.permalink}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium text-zinc-700 transition hover:bg-white/70 hover:text-zinc-950"
                      >
                        Lihat
                        <ExternalLink className="h-3 w-3" />
                      </a>
                    ) : (
                      <span className="text-zinc-400">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
