package ratelimit

import (
	"testing"
	"time"

	"meta-hashtag-views/internal/domain"
)

func TestLimiterSlidingWindowAndCooldown(t *testing.T) {
	now := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	lim := NewLimiter(map[domain.Platform]Policy{
		domain.PlatformTikTok: {
			Limit:           2,
			Window:          10 * time.Minute,
			DefaultCooldown: 5 * time.Minute,
		},
	}).WithClock(func() time.Time { return now })

	ok, st := lim.CheckAndConsume(domain.PlatformTikTok)
	if !ok || st.Remaining != 1 || st.Used != 1 || st.RetryAfterSeconds != 0 {
		t.Fatalf("hit 1 unexpected: ok=%v status=%+v", ok, st)
	}

	now = now.Add(2 * time.Minute)
	ok, st = lim.CheckAndConsume(domain.PlatformTikTok)
	if !ok || st.Remaining != 0 || st.Used != 2 || st.RetryAfterSeconds != 480 {
		t.Fatalf("hit 2 unexpected: ok=%v status=%+v", ok, st)
	}

	ok, st = lim.CheckAndConsume(domain.PlatformTikTok)
	if ok || st.RetryAfterSeconds != 480 || st.CooldownUntil == "" {
		t.Fatalf("hit 3 should be rejected: ok=%v status=%+v", ok, st)
	}

	now = now.Add(8 * time.Minute)
	st = lim.Status(domain.PlatformTikTok)
	if st.Remaining != 1 || st.RetryAfterSeconds != 0 || st.CooldownUntil != "" {
		t.Fatalf("expected 1 slot open after oldest expired: %+v", st)
	}

	st = lim.RecordUpstreamCooldown(domain.PlatformTikTok, 90*time.Second)
	if !st.UpstreamBlocked || st.Remaining != 0 || st.RetryAfterSeconds != 90 {
		t.Fatalf("unexpected upstream cooldown status: %+v", st)
	}

	ok, _ = lim.CheckAndConsume(domain.PlatformTikTok)
	if ok {
		t.Fatalf("expected request blocked during upstream cooldown")
	}

	now = now.Add(91 * time.Second)
	st = lim.Status(domain.PlatformTikTok)
	if st.UpstreamBlocked || st.RetryAfterSeconds != 0 || st.Remaining != 1 {
		t.Fatalf("expected upstream cooldown cleared: %+v", st)
	}
}

func TestFormatDurationID(t *testing.T) {
	cases := map[int]string{
		45:   "45 detik",
		60:   "1 menit",
		125:  "2 menit 5 detik",
		3600: "1 jam",
		4500: "1 jam 15 menit",
	}
	for sec, want := range cases {
		if got := FormatDurationID(sec); got != want {
			t.Errorf("FormatDurationID(%d) = %q, want %q", sec, got, want)
		}
	}
}

func TestLimiterUnlimitedPolicyWithUpstreamCooldown(t *testing.T) {
	now := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	lim := NewLimiter(nil).WithClock(func() time.Time { return now })

	for i := 0; i < 25; i++ {
		ok, st := lim.CheckAndConsume(domain.PlatformTikTok)
		if !ok || !st.Unlimited || st.RetryAfterSeconds != 0 || st.UpstreamBlocked {
			t.Fatalf("call %d unexpectedly blocked or limited: ok=%v status=%+v", i+1, ok, st)
		}
	}

	st := lim.RecordUpstreamCooldown(domain.PlatformTikTok, 90*time.Second)
	if !st.Unlimited || !st.UpstreamBlocked || st.RetryAfterSeconds != 90 || st.CooldownUntil == "" {
		t.Fatalf("unexpected upstream cooldown status: %+v", st)
	}

	ok, st := lim.CheckAndConsume(domain.PlatformTikTok)
	if ok || !st.UpstreamBlocked || st.RetryAfterSeconds != 90 {
		t.Fatalf("expected request blocked during upstream cooldown: ok=%v status=%+v", ok, st)
	}

	now = now.Add(91 * time.Second)
	ok, st = lim.CheckAndConsume(domain.PlatformTikTok)
	if !ok || !st.Unlimited || st.UpstreamBlocked || st.RetryAfterSeconds != 0 {
		t.Fatalf("expected unlimited searches to resume after cooldown: ok=%v status=%+v", ok, st)
	}
}
