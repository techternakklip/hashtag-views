import type { ReactNode } from 'react';

export interface SegmentOption<T extends string> {
  value: T;
  label: ReactNode;
  disabled?: boolean;
  title?: string;
}

interface SegmentedControlProps<T extends string> {
  value: T;
  options: SegmentOption<T>[];
  onChange: (value: T) => void;
  ariaLabel: string;
  size?: 'xs' | 'sm' | 'md';
  role?: 'tablist' | 'group';
}

export function SegmentedControl<T extends string>({
  value,
  options,
  onChange,
  ariaLabel,
  size = 'md',
  role = 'group',
}: SegmentedControlProps<T>) {
  const activeIdx = Math.max(
    0,
    options.findIndex((o) => o.value === value),
  );
  const count = Math.max(1, options.length);

  const containerCls =
    size === 'xs'
      ? 'rounded-full bg-zinc-900/[0.05] border border-white/60 p-0.5 text-xs'
      : size === 'sm'
        ? 'glass-surface rounded-full p-1 text-xs'
        : 'glass-surface rounded-full p-1 text-sm';

  const padPx = size === 'xs' ? 2 : 4;
  const pillCls =
    size === 'xs'
      ? 'glass-pill bottom-0.5 top-0.5 rounded-full'
      : 'glass-pill bottom-1 top-1 rounded-full';

  const btnCls =
    size === 'xs'
      ? 'rounded-full px-3 py-1'
      : size === 'sm'
        ? 'rounded-full px-3.5 py-1.5'
        : 'rounded-full px-4 py-1.5';

  return (
    <div
      role={role}
      aria-label={ariaLabel}
      style={{ gridTemplateColumns: `repeat(${count}, minmax(0, 1fr))` }}
      className={`relative grid ${containerCls}`}
    >
      <span
        aria-hidden="true"
        style={{
          left: `${padPx}px`,
          width: `calc((100% - ${padPx * 2}px) / ${count})`,
          transform: `translateX(${activeIdx * 100}%)`,
        }}
        className={`ease-smooth pointer-events-none absolute transition-transform duration-300 ${pillCls}`}
      />
      {options.map((opt) => {
        const selected = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            role={role === 'tablist' ? 'tab' : undefined}
            aria-selected={role === 'tablist' ? selected : undefined}
            aria-pressed={role === 'group' ? selected : undefined}
            disabled={opt.disabled}
            title={opt.title}
            onClick={() => !opt.disabled && onChange(opt.value)}
            className={`relative z-10 inline-flex items-center justify-center gap-2 whitespace-nowrap font-medium transition-colors duration-200 disabled:cursor-not-allowed disabled:opacity-40 ${btnCls} ${
              selected ? 'text-zinc-950' : 'text-zinc-500 hover:text-zinc-900'
            }`}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
