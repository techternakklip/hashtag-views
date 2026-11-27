package config

import (
	"slices"
	"testing"
)

func TestLoadFeatureFlags(t *testing.T) {
	t.Setenv("ENABLED_PLATFORMS", "")
	t.Setenv("ALLOW_MOCK_MODE", "")

	defCfg := Load()
	wantDef := []string{"instagram", "facebook", "tiktok"}
	if !slices.Equal(defCfg.EnabledPlatforms, wantDef) {
		t.Fatalf("default EnabledPlatforms = %v, want %v", defCfg.EnabledPlatforms, wantDef)
	}
	if !defCfg.AllowMockMode {
		t.Fatalf("default AllowMockMode = false, want true")
	}

	t.Setenv("ENABLED_PLATFORMS", "tiktok")
	t.Setenv("ALLOW_MOCK_MODE", "false")
	prodCfg := Load()
	if !slices.Equal(prodCfg.EnabledPlatforms, []string{"tiktok"}) {
		t.Fatalf("prod EnabledPlatforms = %v, want [tiktok]", prodCfg.EnabledPlatforms)
	}
	if prodCfg.AllowMockMode {
		t.Fatalf("prod AllowMockMode = true, want false")
	}

	t.Setenv("ENABLED_PLATFORMS", " tt , ig , tiktok ")
	t.Setenv("ALLOW_MOCK_MODE", "invalid-bool")
	aliasCfg := Load()
	wantAlias := []string{"tiktok", "instagram"}
	if !slices.Equal(aliasCfg.EnabledPlatforms, wantAlias) {
		t.Fatalf("alias EnabledPlatforms = %v, want %v", aliasCfg.EnabledPlatforms, wantAlias)
	}
	if !aliasCfg.AllowMockMode {
		t.Fatalf("invalid bool fallback = false, want true")
	}

	t.Setenv("ENABLED_PLATFORMS", "invalid,unknown")
	fallbackCfg := Load()
	if !slices.Equal(fallbackCfg.EnabledPlatforms, []string{"tiktok"}) {
		t.Fatalf("fallback EnabledPlatforms = %v, want [tiktok]", fallbackCfg.EnabledPlatforms)
	}
}

func TestLoadHostConfig(t *testing.T) {
	t.Setenv("HOST", "")
	if got := Load().Host; got != DefaultHost {
		t.Fatalf("empty HOST = %q, want %q", got, DefaultHost)
	}

	t.Setenv("HOST", "   ")
	if got := Load().Host; got != DefaultHost {
		t.Fatalf("whitespace HOST = %q, want %q", got, DefaultHost)
	}

	t.Setenv("HOST", " 0.0.0.0 ")
	if got := Load().Host; got != "0.0.0.0" {
		t.Fatalf("trimmed HOST = %q, want %q", got, "0.0.0.0")
	}

	t.Setenv("HOST", "::1")
	if got := Load().Host; got != "::1" {
		t.Fatalf("ipv6 HOST = %q, want %q", got, "::1")
	}
}
