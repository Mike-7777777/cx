package main

import "testing"

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
