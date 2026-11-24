import type { RunRecord } from '../types';
import { formatDateTime, formatNumber } from '../utils/format';

interface RunHistoryProps {
  runs: RunRecord[];
  showMode?: boolean;
}

export function RunHistory({ runs, showMode = true }: RunHistoryProps) {
  if (runs.length === 0) {
    return (
      <div className="py-12 text-center text-xs text-zinc-400">
        Belum ada riwayat pengecekan untuk hashtag ini.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-[540px] w-full text-left text-xs">
        <thead>
          <tr className="glass-subbar border-b border-zinc-900/[0.06] text-[11px] font-medium uppercase tracking-wider text-zinc-500">
            <th className="w-20 py-2.5 pl-6 pr-4">Nomor</th>
            <th className="py-2.5 pr-6">Waktu</th>
            {showMode && <th className="w-28 px-4 py-2.5">Mode</th>}
            <th className="w-28 px-4 py-2.5">Status</th>
            <th className="w-32 px-4 py-2.5 text-right">Video</th>
            <th className="w-44 py-2.5 pl-4 pr-6 text-right">Total Views</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-900/[0.05]">
          {runs.map((run, idx) => (
            <tr key={run.id} className="transition-colors hover:bg-white/45">
              <td className="py-3.5 pl-6 pr-4 tabular-nums text-zinc-500">{idx + 1}</td>
              <td className="py-3.5 pr-6 whitespace-nowrap font-medium tabular-nums text-zinc-900">
                {formatDateTime(run.startedAt)}
              </td>
              {showMode && (
                <td className="px-4 py-3.5 whitespace-nowrap">
                  <span className="rounded-full bg-zinc-900/[0.06] px-2 py-0.5 text-[10px] font-medium uppercase text-zinc-700">
                    {run.mode === 'direct'
                      ? 'Langsung'
                      : run.mode === 'mock'
                        ? 'Simulasi'
                        : 'Live'}
                  </span>
                </td>
              )}
              <td className="px-4 py-3.5 whitespace-nowrap">
                <StatusPill status={run.status} title={run.errorMessage} />
              </td>
              <td className="px-4 py-3.5 text-right whitespace-nowrap tabular-nums text-zinc-700">
                {formatNumber(run.videoItemsCount)}
              </td>
              <td className="py-3.5 pl-4 pr-6 text-right whitespace-nowrap tabular-nums font-semibold text-zinc-950">
                {formatNumber(run.totalViewsSnapshot)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function StatusPill({ status, title }: { status: RunRecord['status']; title?: string }) {
  const label: Record<RunRecord['status'], string> = {
    running: 'Berjalan',
    completed: 'Selesai',
    partial: 'Sebagian',
    failed: 'Gagal',
  };
  const styles: Record<RunRecord['status'], string> = {
    running: 'bg-zinc-900/[0.06] text-zinc-700',
    completed: 'border border-emerald-500/20 bg-emerald-500/12 text-emerald-700',
    partial: 'border border-amber-500/20 bg-amber-500/12 text-amber-700',
    failed: 'border border-rose-500/20 bg-rose-500/12 text-rose-700',
  };
  return (
    <span
      title={title}
      className={`inline-block rounded-full px-2.5 py-0.5 text-[11px] font-medium ${styles[status]}`}
    >
      {label[status]}
    </span>
  );
}
