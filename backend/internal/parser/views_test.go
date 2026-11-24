package parser

import (
	"encoding/json"
	"testing"
)

func TestParseViews(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want *int64
	}{
		{name: "nil returns nil", in: nil, want: nil},
		{name: "empty string returns nil", in: "   ", want: nil},
		{name: "dash returns nil", in: "-", want: nil},
		{name: "negative int returns nil", in: -5, want: nil},
		{name: "zero int returns zero", in: 0, want: ptr(0)},
		{name: "plain int64", in: int64(28299), want: ptr(28299)},
		{name: "float64", in: float64(4200.0), want: ptr(4200)},
		{name: "json number", in: json.Number("91500"), want: ptr(91500)},
		{name: "comma thousands", in: "28,299", want: ptr(28299)},
		{name: "multi comma thousands", in: "1,250,400", want: ptr(1250400)},
		{name: "multi dot thousands", in: "1.250.400", want: ptr(1250400)},
		{name: "english K", in: "12.5K", want: ptr(12500)},
		{name: "english M with label", in: "1.2M views", want: ptr(1200000)},
		{name: "english B", in: "1.1B plays", want: ptr(1100000000)},
		{name: "indonesian rb with comma", in: "3,5 rb", want: ptr(3500)},
		{name: "indonesian jt with comma", in: "1,5 jt tayangan", want: ptr(1500000)},
		{name: "garbage string", in: "hidden", want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseViews(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("ParseViews(%v) = %d, want nil", tc.in, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseViews(%v) = nil, want %d", tc.in, *tc.want)
			}
			if *got != *tc.want {
				t.Fatalf("ParseViews(%v) = %d, want %d", tc.in, *got, *tc.want)
			}
		})
	}
}

func ptr(v int64) *int64 {
	return &v
}
