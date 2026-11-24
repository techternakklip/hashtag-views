package parser

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"meta-hashtag-views/internal/domain"
)

var (
	ErrEmptyHashtag   = errors.New("hashtag tidak boleh kosong")
	ErrInvalidHashtag = errors.New("hashtag hanya boleh berisi huruf, angka, dan underscore tanpa spasi")
)

func NormalizeHashtag(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimLeft(cleaned, "#")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.ToLower(cleaned)

	if cleaned == "" {
		return "", ErrEmptyHashtag
	}

	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		return "", fmt.Errorf("%w: %q", ErrInvalidHashtag, raw)
	}

	return cleaned, nil
}

func ParsePlatforms(raw string) ([]domain.Platform, error) {
	if strings.TrimSpace(raw) == "" {
		return append([]domain.Platform(nil), domain.AllPlatforms...), nil
	}

	seen := make(map[domain.Platform]bool)
	var result []domain.Platform
	for _, token := range strings.Split(raw, ",") {
		if strings.TrimSpace(token) == "" {
			continue
		}
		plat, err := ParsePlatform(token)
		if err != nil {
			return nil, err
		}
		if !seen[plat] {
			seen[plat] = true
			result = append(result, plat)
		}
	}

	if len(result) == 0 {
		return nil, errors.New("minimal satu platform harus dipilih")
	}
	return result, nil
}

func ParsePlatform(raw string) (domain.Platform, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ig", string(domain.PlatformInstagram):
		return domain.PlatformInstagram, nil
	case "fb", string(domain.PlatformFacebook):
		return domain.PlatformFacebook, nil
	case "tt", string(domain.PlatformTikTok):
		return domain.PlatformTikTok, nil
	default:
		return "", fmt.Errorf("platform tidak dikenal: %q (gunakan ig/instagram, fb/facebook, atau tt/tiktok)", raw)
	}
}
