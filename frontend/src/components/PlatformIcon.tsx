import { useId } from 'react';
import { siFacebook, siInstagram, siTiktok } from 'simple-icons';
import type { Platform } from '../types';

interface PlatformIconProps {
  platform: Platform;
  className?: string;
}

export function PlatformIcon({ platform, className = 'h-4 w-4' }: PlatformIconProps) {
  const gradId = useId();

  if (platform === 'instagram') {
    return (
      <svg
        viewBox="0 0 24 24"
        role="img"
        aria-label={siInstagram.title}
        className={className}
      >
        <defs>
          <radialGradient id={gradId} cx="30%" cy="107%" r="150%">
            <stop offset="0%" stopColor="#FDF497" />
            <stop offset="5%" stopColor="#FDF497" />
            <stop offset="45%" stopColor="#FD5949" />
            <stop offset="60%" stopColor="#D6249F" />
            <stop offset="90%" stopColor="#285AEB" />
          </radialGradient>
        </defs>
        <path d={siInstagram.path} fill={`url(#${gradId})`} />
      </svg>
    );
  }

  if (platform === 'tiktok') {
    return (
      <svg
        viewBox="-1.2 -1.2 26.4 26.4"
        role="img"
        aria-label={siTiktok.title}
        className={className}
      >
        <path d={siTiktok.path} fill="#25F4EE" transform="translate(-0.85, -0.65)" />
        <path d={siTiktok.path} fill="#FE2C55" transform="translate(0.85, 0.65)" />
        <path d={siTiktok.path} fill="#09090B" />
      </svg>
    );
  }

  return (
    <svg
      viewBox="0 0 24 24"
      role="img"
      aria-label={siFacebook.title}
      className={className}
    >
      <path d={siFacebook.path} fill={`#${siFacebook.hex}`} />
    </svg>
  );
}
