package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestDisplaySlug(t *testing.T) {
	tests := []struct {
		name string
		s    sessionEntry
		want string
	}{
		{
			name: "slug set",
			s:    sessionEntry{Slug: "fix-statusline-bug", ID: "abc123def456xyz"},
			want: "fix-statusline-bug",
		},
		{
			name: "empty slug, long ID",
			s:    sessionEntry{Slug: "", ID: "abcdef123456xyz789"},
			want: "abcdef123456...",
		},
		{
			name: "empty slug, short ID",
			s:    sessionEntry{Slug: "", ID: "short"},
			want: "short",
		},
		{
			name: "empty slug, exactly 12 chars",
			s:    sessionEntry{Slug: "", ID: "123456789012"},
			want: "123456789012",
		},
		{
			name: "empty slug, 13 chars",
			s:    sessionEntry{Slug: "", ID: "1234567890123"},
			want: "123456789012...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displaySlug(&tt.s)
			if got != tt.want {
				t.Errorf("displaySlug() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResume_HelpFlag(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &resumeCmd{}
	err := cmd.Run(context.Background(), app, []string{"--help"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "cx resume") {
		t.Errorf("help missing usage text: %q", buf.String())
	}
}
