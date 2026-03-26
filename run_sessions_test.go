package main

import (
	"testing"
	"time"
)

func TestShortProjectName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"I--google-drive-homebase", "homebase"},
		{"C--Users-foo-project", "project"},
		{"single", "single"},
		{"a-b-c", "c"},
		{"", ""},
	}

	for _, tt := range tests {
		got := shortProjectName(tt.input)
		if got != tt.want {
			t.Errorf("shortProjectName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{48 * time.Hour, "2d ago"},
		{0, "now"},
		{59 * time.Second, "now"},
		{90 * time.Minute, "1h ago"},
		{25 * time.Hour, "1d ago"},
	}

	for _, tt := range tests {
		got := formatAge(tt.input)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
