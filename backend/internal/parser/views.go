package parser

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var compactCountPattern = regexp.MustCompile(`^\s*([0-9]+(?:[.,][0-9]+)*)\s*(rb|ribu|k|jt|juta|m|miliar|b|t)?(?:\s*(?:views?|plays?|tayangan))?\s*$`)

func ParseViews(raw any) *int64 {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case int:
		return nonNegativePtr(int64(v))
	case int32:
		return nonNegativePtr(int64(v))
	case int64:
		return nonNegativePtr(v)
	case uint64:
		if v > math.MaxInt64 {
			return nil
		}
		val := int64(v)
		return &val
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return nil
		}
		return nonNegativePtr(int64(math.Round(v)))
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return nonNegativePtr(i)
		}
		if f, err := v.Float64(); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) && f >= 0 {
			return nonNegativePtr(int64(math.Round(f)))
		}
		return nil
	case string:
		return parseViewsString(v)
	default:
		return nil
	}
}

func parseViewsString(raw string) *int64 {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" || trimmed == "-" || trimmed == "n/a" || trimmed == "null" {
		return nil
	}

	matches := compactCountPattern.FindStringSubmatch(trimmed)
	if len(matches) == 0 {
		return nil
	}

	numPart := matches[1]
	unit := matches[2]
	multiplier := unitMultiplier(unit)

	if multiplier > 1 {
		normalized := normalizeDecimalString(numPart)
		f, err := strconv.ParseFloat(normalized, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
			return nil
		}
		val := int64(math.Round(f * multiplier))
		return &val
	}

	normalizedInt := stripGroupingSeparators(numPart)
	if i, err := strconv.ParseInt(normalizedInt, 10, 64); err == nil {
		return nonNegativePtr(i)
	}

	normalizedFloat := normalizeDecimalString(numPart)
	if f, err := strconv.ParseFloat(normalizedFloat, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) && f >= 0 {
		val := int64(math.Round(f))
		return &val
	}

	return nil
}

func unitMultiplier(unit string) float64 {
	switch unit {
	case "k", "rb", "ribu":
		return 1_000
	case "m", "jt", "juta":
		return 1_000_000
	case "b", "miliar":
		return 1_000_000_000
	case "t":
		return 1_000_000_000_000
	default:
		return 1
	}
}

func normalizeDecimalString(s string) string {
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")
	if lastComma > lastDot {
		cleaned := strings.ReplaceAll(s[:lastComma], ".", "")
		cleaned = strings.ReplaceAll(cleaned, ",", "")
		return cleaned + "." + s[lastComma+1:]
	}
	return strings.ReplaceAll(s, ",", "")
}

func stripGroupingSeparators(s string) string {
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		parts := strings.Split(s, ",")
		if isGroupedThousands(parts) {
			return strings.Join(parts, "")
		}
	}
	if strings.Contains(s, ".") && !strings.Contains(s, ",") {
		parts := strings.Split(s, ".")
		if len(parts) > 2 && isGroupedThousands(parts) {
			return strings.Join(parts, "")
		}
	}
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		lastComma := strings.LastIndex(s, ",")
		lastDot := strings.LastIndex(s, ".")
		if lastDot > lastComma {
			return strings.ReplaceAll(s, ",", "")
		}
		return strings.ReplaceAll(s, ".", "")
	}
	return s
}

func isGroupedThousands(parts []string) bool {
	if len(parts) < 2 || len(parts[0]) == 0 || len(parts[0]) > 3 {
		return false
	}
	for _, p := range parts[1:] {
		if len(p) != 3 {
			return false
		}
	}
	return true
}

func nonNegativePtr(v int64) *int64 {
	if v < 0 {
		return nil
	}
	return &v
}
