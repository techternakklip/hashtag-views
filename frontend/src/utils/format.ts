import type { Platform } from '../types';

const idNumberFmt = new Intl.NumberFormat('id-ID');
const idCompactLongFmt = new Intl.NumberFormat('id-ID', {
  notation: 'compact',
  compactDisplay: 'long',
  maximumFractionDigits: 1,
});

export function formatPlatformLabel(platform: Platform): string {
  switch (platform) {
    case 'instagram':
      return 'Instagram';
    case 'facebook':
      return 'Facebook';
    case 'tiktok':
      return 'TikTok';
  }
}

export function formatNumber(n: number | null | undefined): string {
  if (n === null || n === undefined) return '—';
  return idNumberFmt.format(n);
}

export function formatCompact(n: number): string {
  if (n >= 1_000_000_000_000) return `${(n / 1_000_000_000_000).toFixed(1).replace('.', ',')} T`;
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1).replace('.', ',')} M`;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2).replace('.', ',')} jt`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1).replace('.', ',')} rb`;
  return String(n);
}

export function formatCompactLong(n: number): string {
  return idCompactLongFmt.format(n);
}

export function formatDelta(n: number | null | undefined): string {
  if (n === null || n === undefined) return '';
  const prefix = n > 0 ? '+' : '';
  return `${prefix}${idNumberFmt.format(n)}`;
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatShortDate(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}

export function formatShortTime(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleTimeString('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatDurationID(totalSeconds: number): string {
  const sec = Math.max(0, Math.ceil(totalSeconds));
  const hours = Math.floor(sec / 3600);
  const mins = Math.floor((sec % 3600) / 60);
  const rem = sec % 60;
  if (hours > 0) {
    return mins > 0 ? `${hours} jam ${mins} mnt` : `${hours} jam`;
  }
  if (mins > 0) {
    return rem > 0 ? `${mins} mnt ${rem} dtk` : `${mins} mnt`;
  }
  return `${rem} dtk`;
}

export function formatDurationClock(totalSeconds: number): string {
  const sec = Math.max(0, Math.ceil(totalSeconds));
  const hours = Math.floor(sec / 3600);
  const mins = Math.floor((sec % 3600) / 60);
  const rem = sec % 60;
  if (hours > 0) {
    return `${hours}j ${String(mins).padStart(2, '0')}m`;
  }
  return `${String(mins).padStart(2, '0')}:${String(rem).padStart(2, '0')}`;
}

export function formatWindowLabel(windowSeconds: number): string {
  if (windowSeconds >= 3600 && windowSeconds % 3600 === 0) {
    return `${windowSeconds / 3600} jam`;
  }
  if (windowSeconds >= 60 && windowSeconds % 60 === 0) {
    return `${windowSeconds / 60} menit`;
  }
  return `${windowSeconds} detik`;
}
