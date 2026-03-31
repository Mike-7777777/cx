package main

import (
	"strings"
	"testing"
	"time"

	"github.com/Mike-7777777/cx/internal/platform"
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

// ---------- STATE MACHINE TESTS ----------

func newTestState() *dashState {
	return &dashState{
		view:     viewMain,
		section:  0,
		useColor: false,
		interval: 5 * time.Second,
		data:     &dashboardData{},
	}
}

func TestHandleDashKey_MainNavigation(t *testing.T) {
	state := newTestState()

	// Press Down 3 times → section 3.
	for i := 0; i < 3; i++ {
		handleDashKey(state, platform.KeyDown)
	}
	if state.section != 3 {
		t.Errorf("after 3x Down: section = %d, want 3", state.section)
	}

	// Press Up → section 2.
	handleDashKey(state, platform.KeyUp)
	if state.section != 2 {
		t.Errorf("after Up: section = %d, want 2", state.section)
	}
}

func TestHandleDashKey_EnterSubView(t *testing.T) {
	state := newTestState()
	state.section = sectionSessions

	handleDashKey(state, platform.KeyEnter)

	if state.view != viewSub {
		t.Errorf("view = %d, want viewSub (%d)", state.view, viewSub)
	}
	if state.subCursor != 0 {
		t.Errorf("subCursor = %d, want 0", state.subCursor)
	}
}

func TestHandleDashKey_SubViewBack(t *testing.T) {
	state := newTestState()
	state.view = viewSub
	state.section = sectionSessions

	action := handleDashKey(state, platform.KeyQ)

	if state.view != viewMain {
		t.Errorf("view = %d, want viewMain (%d)", state.view, viewMain)
	}
	if action != dashActionNone {
		t.Errorf("action = %d, want dashActionNone (%d)", action, dashActionNone)
	}
}

func TestHandleDashKey_EscSubView(t *testing.T) {
	state := newTestState()
	state.view = viewSub
	state.section = sectionAccounts

	action := handleDashKey(state, platform.KeyEsc)

	if state.view != viewMain {
		t.Errorf("view = %d, want viewMain (%d)", state.view, viewMain)
	}
	if action != dashActionNone {
		t.Errorf("action = %d, want dashActionNone (%d)", action, dashActionNone)
	}
}

func TestHandleDashKey_ResumeAction(t *testing.T) {
	state := newTestState()
	state.view = viewSub
	state.section = sectionSessions
	state.sessionList = []sessionEntry{
		{ID: "sess-1", Account: "main", Project: "test"},
	}

	action := handleDashKey(state, platform.KeyR)

	if action != dashActionResume {
		t.Errorf("action = %d, want dashActionResume (%d)", action, dashActionResume)
	}
}

func TestHandleDashKey_SwitchAction(t *testing.T) {
	state := newTestState()
	state.view = viewSub
	state.section = sectionAccounts
	state.accountList = []string{"acct-a", "acct-b"}
	state.subCursor = 1

	action := handleDashKey(state, platform.KeyS)

	if action != dashActionSwitch {
		t.Errorf("action = %d, want dashActionSwitch (%d)", action, dashActionSwitch)
	}
}

func TestHandleDashKey_RInNonSessionsView(t *testing.T) {
	state := newTestState()
	state.view = viewSub
	state.section = sectionROI

	action := handleDashKey(state, platform.KeyR)

	if action != dashActionNone {
		t.Errorf("action = %d, want dashActionNone (%d)", action, dashActionNone)
	}
}

func TestHandleDashKey_BoundsCheck(t *testing.T) {
	// At max section, Down should stay.
	state := newTestState()
	state.section = sectionCount - 1

	handleDashKey(state, platform.KeyDown)
	if state.section != sectionCount-1 {
		t.Errorf("section = %d, want %d (should not exceed max)", state.section, sectionCount-1)
	}

	// At section 0, Up should stay.
	state.section = 0
	handleDashKey(state, platform.KeyUp)
	if state.section != 0 {
		t.Errorf("section = %d, want 0 (should not go below 0)", state.section)
	}
}

func TestHandleDashKey_MainQuit(t *testing.T) {
	state := newTestState()

	action := handleDashKey(state, platform.KeyQ)

	if action != dashActionQuit {
		t.Errorf("action = %d, want dashActionQuit (%d)", action, dashActionQuit)
	}
}

func TestSubViewRowCount(t *testing.T) {
	// Sessions with 3 entries.
	state := newTestState()
	state.section = sectionSessions
	state.sessionList = []sessionEntry{
		{ID: "s1"}, {ID: "s2"}, {ID: "s3"},
	}
	if got := subViewRowCount(state); got != 3 {
		t.Errorf("sectionSessions with 3 entries: got %d, want 3", got)
	}

	// ROI → 0 (no selectable rows).
	state.section = sectionROI
	if got := subViewRowCount(state); got != 0 {
		t.Errorf("sectionROI: got %d, want 0", got)
	}

	// Accounts with 2 entries.
	state.section = sectionAccounts
	state.accountList = []string{"a", "b"}
	if got := subViewRowCount(state); got != 2 {
		t.Errorf("sectionAccounts with 2 entries: got %d, want 2", got)
	}
}

func TestHighlightSection(t *testing.T) {
	input := "║  ACCOUNTS\n║  data\n"
	got := highlightSection(input)
	if !strings.Contains(got, "▸") {
		t.Errorf("highlightSection output should contain ▸, got %q", got)
	}
	// Only the first occurrence should be replaced.
	count := strings.Count(got, "▸")
	if count != 1 {
		t.Errorf("expected exactly 1 ▸ marker, got %d", count)
	}
}

func TestHeatmapCell(t *testing.T) {
	// cost=0 → contains ░
	cell0 := heatmapCell(0, 100, false)
	if !strings.Contains(cell0, "░") {
		t.Errorf("cost=0: expected ░, got %q", cell0)
	}

	// cost=10, max=100 (ratio=0.1, low) → contains ▒
	cellLow := heatmapCell(10, 100, false)
	if !strings.Contains(cellLow, "▒") {
		t.Errorf("cost=10/max=100: expected ▒, got %q", cellLow)
	}

	// cost=90, max=100 (ratio=0.9, high) → contains █
	cellHigh := heatmapCell(90, 100, false)
	if !strings.Contains(cellHigh, "█") {
		t.Errorf("cost=90/max=100: expected █, got %q", cellHigh)
	}
}
