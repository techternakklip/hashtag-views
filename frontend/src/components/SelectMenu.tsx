import { useEffect, useRef, useState } from 'react';
import { Check, ChevronDown } from 'lucide-react';

export interface SelectOption<T extends string | number> {
  value: T;
  label: string;
}

interface SelectMenuProps<T extends string | number> {
  value: T;
  options: SelectOption<T>[];
  onChange: (value: T) => void;
  ariaLabel: string;
  size?: 'sm' | 'md' | 'lg';
  align?: 'left' | 'right';
}

export function SelectMenu<T extends string | number>({
  value,
  options,
  onChange,
  ariaLabel,
  size = 'md',
  align = 'left',
}: SelectMenuProps<T>) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const handlePointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  const current = options.find((o) => o.value === value) ?? options[0];
  const sizeClass =
    size === 'lg'
      ? 'h-11 rounded-full px-4 text-sm'
      : size === 'sm'
        ? 'h-8 rounded-full px-3 text-xs'
        : 'h-9 rounded-full px-3.5 text-xs';

  return (
    <div ref={rootRef} className="relative inline-block">
      <button
        type="button"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className={`glass-surface inline-flex ${sizeClass} items-center justify-between gap-2 font-medium text-zinc-700 transition hover:bg-white/75 hover:text-zinc-950 focus:border-[#007AFF]/45 focus:outline-none focus:ring-2 focus:ring-[#007AFF]/18`}
      >
        <span className="whitespace-nowrap">{current?.label}</span>
        <ChevronDown
          className={`h-3.5 w-3.5 shrink-0 text-zinc-400 transition-transform ${open ? 'rotate-180' : ''}`}
        />
      </button>

      {open && (
        <ul
          role="listbox"
          aria-label={ariaLabel}
          className={`animate-dropdown-in glass-popover absolute z-30 mt-1.5 min-w-[168px] overflow-hidden rounded-2xl p-1.5 ${
            align === 'right' ? 'right-0' : 'left-0'
          }`}
        >
          {options.map((opt) => {
            const selected = opt.value === value;
            return (
              <li key={String(opt.value)}>
                <button
                  type="button"
                  role="option"
                  aria-selected={selected}
                  onClick={() => {
                    onChange(opt.value);
                    setOpen(false);
                  }}
                  className={`flex w-full items-center justify-between gap-2 rounded-xl px-2.5 py-1.5 text-left text-xs transition ${
                    selected
                      ? 'bg-[#007AFF]/[0.09] font-medium text-zinc-950'
                      : 'text-zinc-600 hover:bg-zinc-900/[0.04] hover:text-zinc-950'
                  }`}
                >
                  <span className="whitespace-nowrap">{opt.label}</span>
                  {selected && <Check className="h-3.5 w-3.5 text-[#007AFF]" />}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
