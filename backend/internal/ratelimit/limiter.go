package ratelimit

import (
	"fmt"
	"math"
	"sync"
	"time"

	"meta-hashtag-views/internal/domain"
)

type Policy struct {
	Limit           int
	Window          time.Duration
	DefaultCooldown time.Duration
	Unlimited       bool
	Note            string
}

type Status struct {
	Platform          domain.Platform `json:"platform"`
	Unlimited         bool            `json:"unlimited,omitempty"`
	Limit             int             `json:"limit"`
	Remaining         int             `json:"remaining"`
	Used              int             `json:"used"`
	WindowSeconds     int             `json:"windowSeconds"`
	ResetAt           string          `json:"resetAt,omitempty"`
	FullResetAt       string          `json:"fullResetAt,omitempty"`
	CooldownUntil     string          `json:"cooldownUntil,omitempty"`
	RetryAfterSeconds int             `json:"retryAfterSeconds"`
	UpstreamBlocked   bool            `json:"upstreamBlocked,omitempty"`
	Note              string          `json:"note,omitempty"`
}

type bucket struct {
	policy        Policy
	hits          []time.Time
	cooldownUntil time.Time
	upstreamBlock bool
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[domain.Platform]*bucket
	nowFn   func() time.Time
}

func DefaultPolicies() map[domain.Platform]Policy {
	return map[domain.Platform]Policy{
		domain.PlatformTikTok: {
			Unlimited:       true,
			DefaultCooldown: 5 * time.Minute,
			Note:            "Tanpa batas lokal; cooldown aktif hanya jika TikTok membatasi (HTTP 429)",
		},
		domain.PlatformInstagram: {
			Limit:           20,
			Window:          60 * time.Minute,
			DefaultCooldown: 5 * time.Minute,
			Note:            "Batas aman penarikan Live Instagram",
		},
		domain.PlatformFacebook: {
			Limit:           20,
			Window:          60 * time.Minute,
			DefaultCooldown: 5 * time.Minute,
			Note:            "Batas aman penarikan Live Facebook",
		},
	}
}

func NewLimiter(policies map[domain.Platform]Policy) *Limiter {
	if len(policies) == 0 {
		policies = DefaultPolicies()
	}
	buckets := make(map[domain.Platform]*bucket, len(policies))
	for p, pol := range policies {
		if !pol.Unlimited {
			if pol.Limit <= 0 {
				pol.Limit = 10
			}
			if pol.Window <= 0 {
				pol.Window = 10 * time.Minute
			}
		}
		if pol.DefaultCooldown <= 0 {
			pol.DefaultCooldown = 5 * time.Minute
		}
		buckets[p] = &bucket{policy: pol}
	}
	return &Limiter{
		buckets: buckets,
		nowFn:   func() time.Time { return time.Now().UTC() },
	}
}

func (l *Limiter) WithClock(fn func() time.Time) *Limiter {
	if fn != nil {
		l.mu.Lock()
		l.nowFn = fn
		l.mu.Unlock()
	}
	return l
}

func (l *Limiter) CheckAndConsume(platform domain.Platform) (bool, Status) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFn()
	b := l.bucketFor(platform)
	pruneBucket(b, now)

	if now.Before(b.cooldownUntil) {
		return false, buildStatus(platform, b, now)
	}
	if b.policy.Unlimited {
		return true, buildStatus(platform, b, now)
	}
	if len(b.hits) >= b.policy.Limit {
		return false, buildStatus(platform, b, now)
	}

	b.hits = append(b.hits, now)
	return true, buildStatus(platform, b, now)
}

func (l *Limiter) RecordUpstreamCooldown(platform domain.Platform, retryAfter time.Duration) Status {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFn()
	b := l.bucketFor(platform)
	pruneBucket(b, now)

	if retryAfter <= 0 {
		retryAfter = b.policy.DefaultCooldown
	}
	until := now.Add(retryAfter)
	if until.After(b.cooldownUntil) {
		b.cooldownUntil = until
	}
	b.upstreamBlock = true
	return buildStatus(platform, b, now)
}

func (l *Limiter) Status(platform domain.Platform) Status {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFn()
	b := l.bucketFor(platform)
	pruneBucket(b, now)
	return buildStatus(platform, b, now)
}

func (l *Limiter) AllStatuses() map[domain.Platform]Status {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFn()
	out := make(map[domain.Platform]Status, len(domain.AllPlatforms))
	for _, p := range domain.AllPlatforms {
		b := l.bucketFor(p)
		pruneBucket(b, now)
		out[p] = buildStatus(p, b, now)
	}
	return out
}

func FormatDurationID(seconds int) string {
	if seconds <= 0 {
		return "0 detik"
	}
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("%d jam %d menit", hours, mins)
		}
		return fmt.Sprintf("%d jam", hours)
	}
	if mins > 0 {
		if secs > 0 {
			return fmt.Sprintf("%d menit %d detik", mins, secs)
		}
		return fmt.Sprintf("%d menit", mins)
	}
	return fmt.Sprintf("%d detik", secs)
}

func (l *Limiter) bucketFor(platform domain.Platform) *bucket {
	if b, ok := l.buckets[platform]; ok {
		return b
	}
	def := DefaultPolicies()[platform]
	if !def.Unlimited && def.Limit <= 0 {
		def = Policy{Limit: 10, Window: 10 * time.Minute, DefaultCooldown: 5 * time.Minute}
	}
	if def.DefaultCooldown <= 0 {
		def.DefaultCooldown = 5 * time.Minute
	}
	b := &bucket{policy: def}
	l.buckets[platform] = b
	return b
}

func pruneBucket(b *bucket, now time.Time) {
	if !b.policy.Unlimited && b.policy.Window > 0 {
		cutoff := now.Add(-b.policy.Window)
		idx := 0
		for idx < len(b.hits) && !b.hits[idx].After(cutoff) {
			idx++
		}
		if idx > 0 {
			b.hits = append([]time.Time(nil), b.hits[idx:]...)
		}
	}
	if !b.cooldownUntil.IsZero() && !now.Before(b.cooldownUntil) {
		b.cooldownUntil = time.Time{}
		b.upstreamBlock = false
	}
}

func buildStatus(platform domain.Platform, b *bucket, now time.Time) Status {
	if b.policy.Unlimited {
		st := Status{
			Platform:  platform,
			Unlimited: true,
			Note:      b.policy.Note,
		}
		if !b.cooldownUntil.IsZero() && now.Before(b.cooldownUntil) {
			st.UpstreamBlocked = b.upstreamBlock
			st.CooldownUntil = b.cooldownUntil.UTC().Format(time.RFC3339)
			st.RetryAfterSeconds = int(math.Ceil(b.cooldownUntil.Sub(now).Seconds()))
			if st.RetryAfterSeconds < 1 {
				st.RetryAfterSeconds = 1
			}
		}
		return st
	}

	used := len(b.hits)
	remaining := b.policy.Limit - used
	if remaining < 0 {
		remaining = 0
	}

	st := Status{
		Platform:      platform,
		Limit:         b.policy.Limit,
		Remaining:     remaining,
		Used:          used,
		WindowSeconds: int(b.policy.Window.Seconds()),
		Note:          b.policy.Note,
	}

	if used > 0 {
		nextFree := b.hits[0].Add(b.policy.Window)
		fullFree := b.hits[used-1].Add(b.policy.Window)
		st.ResetAt = nextFree.UTC().Format(time.RFC3339)
		st.FullResetAt = fullFree.UTC().Format(time.RFC3339)
	}

	var effectiveCooldown time.Time
	if !b.cooldownUntil.IsZero() && now.Before(b.cooldownUntil) {
		effectiveCooldown = b.cooldownUntil
		st.UpstreamBlocked = b.upstreamBlock
		st.Remaining = 0
	} else if remaining == 0 && used > 0 {
		effectiveCooldown = b.hits[0].Add(b.policy.Window)
	}

	if !effectiveCooldown.IsZero() && now.Before(effectiveCooldown) {
		st.CooldownUntil = effectiveCooldown.UTC().Format(time.RFC3339)
		st.RetryAfterSeconds = int(math.Ceil(effectiveCooldown.Sub(now).Seconds()))
		if st.RetryAfterSeconds < 1 {
			st.RetryAfterSeconds = 1
		}
	}

	return st
}
