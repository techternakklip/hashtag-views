package parser

import (
	"testing"

	"meta-hashtag-views/internal/domain"
)

func TestNormalizeHashtag(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "#GTA", want: "gta"},
		{in: "  ##gta_vi  ", want: "gta_vi"},
		{in: "ramadan2026", want: "ramadan2026"},
		{in: "", wantErr: true},
		{in: "   ###  ", wantErr: true},
		{in: "gta sa", wantErr: true},
		{in: "gta-online", wantErr: true},
	}

	for _, tc := range tests {
		got, err := NormalizeHashtag(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizeHashtag(%q) expected error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeHashtag(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeHashtag(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParsePlatforms(t *testing.T) {
	got, err := ParsePlatforms("ig,fb,tt,ig")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || got[0] != domain.PlatformInstagram || got[1] != domain.PlatformFacebook || got[2] != domain.PlatformTikTok {
		t.Fatalf("got %v, want [instagram facebook tiktok]", got)
	}

	if _, err := ParsePlatforms("snapchat"); err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}
