package main

import (
	"strings"
	"testing"
)

func TestShortModelName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"claude-opus-4-20250514", "opus"},
		{"claude-sonnet-4-6-20250514", "sonnet"},
		{"claude-haiku-4-5-20251001", "haiku"},
		{"gpt-4", "gpt"},             // unknown: returns second-to-last segment
		{"singleword", "singleword"}, // single segment: returns as-is
		{"", ""},                     // empty
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortModelName(tt.input)
			if got != tt.want {
				t.Errorf("shortModelName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct{ input, want string }{
		{"/home/user/projects/myapp", "myapp"},
		{"C:\\Users\\test\\project", "project"},
		{"/single", "single"},
		{"", ""},
		{"/trailing/slash/", "slash"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortenPath(tt.input)
			if got != tt.want {
				t.Errorf("shortenPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPadLine(t *testing.T) {
	got := padLine("  hello")
	if !strings.HasPrefix(got, "║") {
		t.Error("should start with ║")
	}
	if !strings.HasSuffix(got, "║\n") {
		t.Error("should end with ║ and newline")
	}
	if !strings.Contains(got, "hello") {
		t.Error("should contain the content")
	}
}

func TestEmptyLine(t *testing.T) {
	got := emptyLine()
	if !strings.HasPrefix(got, "║") {
		t.Error("should start with ║")
	}
	if !strings.HasSuffix(got, "║\n") {
		t.Error("should end with ║ and newline")
	}
	// Should be exactly dashboardBoxWidth spaces between the borders.
	inner := got[len("║") : len(got)-len("║\n")]
	if len(inner) != dashboardBoxWidth {
		t.Errorf("inner width = %d, want %d", len(inner), dashboardBoxWidth)
	}
	if strings.TrimSpace(inner) != "" {
		t.Error("emptyLine inner content should be all spaces")
	}
}

func TestDashboardData_Empty(t *testing.T) {
	data := &dashboardData{}
	if len(data.entries) != 0 {
		t.Error("empty data should have no entries")
	}
	if len(data.configDirs) != 0 {
		t.Error("empty data should have no configDirs")
	}
}

func TestBoxTop(t *testing.T) {
	got := boxTop()
	if !strings.HasPrefix(got, "╔") {
		t.Error("should start with ╔")
	}
	if !strings.HasSuffix(got, "╗\n") {
		t.Error("should end with ╗ and newline")
	}
}

func TestBoxMid(t *testing.T) {
	got := boxMid()
	if !strings.HasPrefix(got, "╠") {
		t.Error("should start with ╠")
	}
	if !strings.HasSuffix(got, "╣\n") {
		t.Error("should end with ╣ and newline")
	}
}

func TestBoxBottom(t *testing.T) {
	got := boxBottom()
	if !strings.HasPrefix(got, "╚") {
		t.Error("should start with ╚")
	}
	if !strings.HasSuffix(got, "╝\n") {
		t.Error("should end with ╝ and newline")
	}
}
