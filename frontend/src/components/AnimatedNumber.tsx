import { useEffect, useRef, useState } from 'react';
import { formatNumber } from '../utils/format';

interface AnimatedNumberProps {
  value: number;
  durationMs?: number;
  className?: string;
}

export function AnimatedNumber({ value, durationMs = 420, className }: AnimatedNumberProps) {
  const [display, setDisplay] = useState(value);
  const fromRef = useRef(value);

  useEffect(() => {
    const startVal = fromRef.current;
    if (startVal === value) {
      setDisplay(value);
      return;
    }

    if (
      typeof window !== 'undefined' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    ) {
      fromRef.current = value;
      setDisplay(value);
      return;
    }

    let frameId = 0;
    const startTime = performance.now();

    const step = (now: number) => {
      const elapsed = now - startTime;
      const progress = Math.min(1, elapsed / durationMs);
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = Math.round(startVal + (value - startVal) * eased);
      setDisplay(current);

      if (progress < 1) {
        frameId = window.requestAnimationFrame(step);
      } else {
        fromRef.current = value;
      }
    };

    frameId = window.requestAnimationFrame(step);
    return () => {
      window.cancelAnimationFrame(frameId);
      fromRef.current = value;
    };
  }, [value, durationMs]);

  return <span className={className}>{formatNumber(display)}</span>;
}
