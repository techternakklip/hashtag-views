import { Clock } from 'lucide-react';
import type { Platform, RateLimitStatus } from '../types';
import { formatDurationID, formatPlatformLabel, formatWindowLabel } from '../utils/format';

interface RateLimitBarProps {
  platform: Platform;
  status?: RateLimitStatus;
  isSimulated: boolean;
  isDetailed: boolean;
  cooldownSec: number;
  resetSec: number;
}

export function RateLimitBar({
  platform,
  status,
  isSimulated,
  isDetailed,
  cooldownSec,
  resetSec,
}: RateLimitBarProps) {
  if (!status) return null;

  const platformLabel = formatPlatformLabel(platform);
  const windowLabel = formatWindowLabel(status.windowSeconds);
  const alignClass = isDetailed ? 'justify-start' : 'justify-center';

  if (isSimulated) {
    return (
      <div
        className={`mt-2.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-zinc-500 ${alignClass}`}
      >
        <span className="inline-flex items-center gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-zinc-400" />
          <span>Mode simulasi lokal (tanpa limit)</span>
        </span>
        <span aria-hidden="true">·</span>
        <span>
          Batas mode Live: {status.limit} cek / {windowLabel}
        </span>
      </div>
    );
  }

  const isBlocked = cooldownSec > 0;
  if (status.unlimited && !isBlocked) {
    return null;
  }

  const isLow = !isBlocked && status.remaining <= Math.max(2, Math.floor(status.limit * 0.2));
  const usedRatio = status.limit > 0 ? Math.min(100, Math.round((status.remaining / status.limit) * 100)) : 0;

  if (isBlocked) {
    return (
      <div className={`mt-2.5 flex flex-wrap items-center gap-2 text-xs ${alignClass}`}>
        <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-300/60 bg-amber-50/70 px-2.5 py-0.5 font-medium text-amber-800 backdrop-blur-md">
          <Clock className="h-3 w-3 shrink-0" />
          <span>
            {status.upstreamBlocked
              ? `Dibatasi ${platformLabel}`
              : `Kuota habis (0/${status.limit})`}
          </span>
          <span aria-hidden="true">·</span>
          <span className="tabular-nums">Cooldown {formatDurationID(cooldownSec)}</span>
        </span>
      </div>
    );
  }

  return (
    <div
      className={`mt-2.5 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs text-zinc-600 ${alignClass}`}
      title={status.note}
    >
      <span className="inline-flex items-center gap-1.5">
        <span
          className={`h-1.5 w-1.5 rounded-full ${
            isLow ? 'bg-amber-500' : 'bg-emerald-500'
          }`}
        />
        <span>
          Sisa{' '}
          <strong className={`tabular-nums font-medium ${isLow ? 'text-amber-700' : 'text-zinc-900'}`}>
            {status.remaining}/{status.limit}
          </strong>{' '}
          pencarian
        </span>
      </span>

      {isDetailed && (
        <div
          aria-hidden="true"
          className="h-1 w-14 overflow-hidden rounded-full bg-zinc-900/10"
        >
          <div
            style={{ width: `${usedRatio}%` }}
            className={`ease-smooth h-full rounded-full transition-[width] duration-300 ${
              isLow ? 'bg-amber-500' : 'bg-[#007AFF]/85'
            }`}
          />
        </div>
      )}

      <span className="text-zinc-400" aria-hidden="true">
        ·
      </span>

      {resetSec > 0 ? (
        <span className="inline-flex items-center gap-1 tabular-nums text-zinc-500">
          <Clock className="h-3 w-3 shrink-0" />
          <span>Reset slot dalam {formatDurationID(resetSec)}</span>
        </span>
      ) : (
        <span className="text-zinc-500">
          Batas aman {status.limit} cek / {windowLabel}
        </span>
      )}
    </div>
  );
}
