package config

import (
	"testing"
)

func TestParseDevIDs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []int64
	}{
		{"empty", "", nil},
		{"single", "123", []int64{123}},
		{"multiple", "123,456,789", []int64{123, 456, 789}},
		{"with spaces", " 123 , 456 , 789 ", []int64{123, 456, 789}},
		{"invalid entries skipped", "123,abc,456", []int64{123, 456}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDevIDs(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("parseDevIDs(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("parseDevIDs(%q)[%d] = %d, want %d", tt.raw, i, v, tt.want[i])
				}
			}
		})
	}
}
