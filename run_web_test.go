package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Mike-7777777/cx/internal/usage"
)

// --- buildSessionProjectMap ---

func TestBuildSessionProjectMap(t *testing.T) {
	entries := []usage.Entry{
		{SessionID: "s1", ProjectPath: "I--drive-projectA"},
		{SessionID: "s1", ProjectPath: "I--drive-projectA"},
		{SessionID: "s1", ProjectPath: "I--drive-projectB"},
		{SessionID: "s2", ProjectPath: "I--drive-projectC"},
		{SessionID: "", ProjectPath: "I--drive-ignored"}, // empty session ID
		{SessionID: "s3", ProjectPath: ""},               // empty project
	}

	m := buildSessionProjectMap(entries)

	if m["s1"] != "projectA" {
		t.Errorf("s1: got %q, want %q", m["s1"], "projectA")
	}
	if m["s2"] != "projectC" {
		t.Errorf("s2: got %q, want %q", m["s2"], "projectC")
	}
	if _, ok := m["s3"]; ok {
		t.Error("s3 with empty project should not be in map")
	}
	if _, ok := m[""]; ok {
		t.Error("empty session ID should not be in map")
	}
}

func TestBuildSessionProjectMap_Empty(t *testing.T) {
	m := buildSessionProjectMap(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestBuildSessionProjectMap_MostCommonProject(t *testing.T) {
	entries := []usage.Entry{
		{SessionID: "s1", ProjectPath: "I--drive-alpha"},
		{SessionID: "s1", ProjectPath: "I--drive-beta"},
		{SessionID: "s1", ProjectPath: "I--drive-beta"},
		{SessionID: "s1", ProjectPath: "I--drive-beta"},
	}

	m := buildSessionProjectMap(entries)
	if m["s1"] != "beta" {
		t.Errorf("s1: got %q, want %q (most common)", m["s1"], "beta")
	}
}

// --- lookupProject ---

func TestLookupProject(t *testing.T) {
	m := map[string]string{"s1": "myproject"}

	if got := lookupProject(m, "s1"); got != "myproject" {
		t.Errorf("existing key: got %q, want %q", got, "myproject")
	}
	if got := lookupProject(m, "unknown"); got != "--" {
		t.Errorf("missing key: got %q, want %q", got, "--")
	}
	if got := lookupProject(nil, "s1"); got != "--" {
		t.Errorf("nil map: got %q, want %q", got, "--")
	}
}

// --- API /api/ready ---

func TestAPIReady_BeforeRefresh(t *testing.T) {
	wc := &webCache{} // lastScan is zero

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		wc.mu.RLock()
		ready := !wc.lastScan.IsZero()
		wc.mu.RUnlock()
		writeJSON(w, map[string]bool{"ready": ready})
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/ready", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]bool
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["ready"] {
		t.Error("expected ready=false before refresh")
	}
}

func TestAPIReady_AfterRefresh(t *testing.T) {
	wc := &webCache{}
	wc.lastScan = time.Now()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		wc.mu.RLock()
		ready := !wc.lastScan.IsZero()
		wc.mu.RUnlock()
		writeJSON(w, map[string]bool{"ready": ready})
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/ready", nil))

	var resp map[string]bool
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp["ready"] {
		t.Error("expected ready=true after refresh")
	}
}

// --- API /api/sessions ---

// newTestWebCache creates a webCache populated with n fake sessions.
func newTestWebCache(n int) *webCache {
	sessions := make([]usage.SessionReport, n)
	projMap := make(map[string]string)
	base := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)

	for i := 0; i < n; i++ {
		sid := "session-" + string(rune('A'+i))
		sessions[i] = usage.SessionReport{
			SessionID: sid,
			StartTime: base.Add(-time.Duration(i) * time.Hour),
			Summary: usage.UsageSummary{
				TotalTokens: int64((i + 1) * 1000),
				CostUSD:     float64(i+1) * 0.05,
			},
		}
		projMap[sid] = "proj" + string(rune('A'+i))
	}

	return &webCache{
		sessions:     sessions,
		sessProjects: projMap,
		lastScan:     time.Now(),
	}
}

// buildSessionsHandler returns the /api/sessions handler extracted from Run.
func buildSessionsHandler(wc *webCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wc.mu.RLock()
		sessions := wc.sessions
		projMap := wc.sessProjects
		wc.mu.RUnlock()

		limit := 10
		offset := 0
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		if s := r.URL.Query().Get("offset"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				offset = n
			}
		}

		total := len(sessions)
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		paginated := sessions[offset:end]

		resp := apiSessionsResponse{
			Sessions: make([]apiSession, 0, len(paginated)),
			Total:    total,
			HasMore:  end < total,
		}
		for _, sr := range paginated {
			sid := sr.SessionID
			if len(sid) > 12 {
				sid = sid[:12]
			}
			resp.Sessions = append(resp.Sessions, apiSession{
				SessionID:   sid,
				Project:     lookupProject(projMap, sr.SessionID),
				StartTime:   sr.StartTime.Format(time.RFC3339),
				TotalTokens: sr.Summary.TotalTokens,
				CostUSD:     sr.Summary.CostUSD,
			})
		}
		writeJSON(w, resp)
	}
}

func TestAPISessionsHandler_DefaultPagination(t *testing.T) {
	wc := newTestWebCache(5)
	handler := buildSessionsHandler(wc)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/sessions", nil))

	var resp apiSessionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Total != 5 {
		t.Errorf("total = %d, want 5", resp.Total)
	}
	if len(resp.Sessions) != 5 {
		t.Errorf("sessions count = %d, want 5", len(resp.Sessions))
	}
	if resp.HasMore {
		t.Error("expected has_more=false for 5 sessions with default limit=10")
	}
}

func TestAPISessionsHandler_LimitAndOffset(t *testing.T) {
	wc := newTestWebCache(5)
	handler := buildSessionsHandler(wc)

	// limit=2, offset=0 → first 2 sessions, has_more=true
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/sessions?limit=2&offset=0", nil))

	var resp apiSessionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Sessions) != 2 {
		t.Errorf("sessions count = %d, want 2", len(resp.Sessions))
	}
	if !resp.HasMore {
		t.Error("expected has_more=true when limit=2 and total=5")
	}
	if resp.Total != 5 {
		t.Errorf("total = %d, want 5", resp.Total)
	}
}

func TestAPISessionsHandler_OffsetPastEnd(t *testing.T) {
	wc := newTestWebCache(5)
	handler := buildSessionsHandler(wc)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/sessions?offset=100", nil))

	var resp apiSessionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Sessions) != 0 {
		t.Errorf("sessions count = %d, want 0 for offset past end", len(resp.Sessions))
	}
	if resp.HasMore {
		t.Error("expected has_more=false for offset past end")
	}
	if resp.Total != 5 {
		t.Errorf("total = %d, want 5", resp.Total)
	}
}

func TestAPISessionsHandler_SessionIDTruncation(t *testing.T) {
	wc := &webCache{
		sessions: []usage.SessionReport{
			{
				SessionID: "abcdefghijklmnop", // 16 chars, should truncate to 12
				StartTime: time.Now(),
				Summary:   usage.UsageSummary{TotalTokens: 500},
			},
		},
		sessProjects: map[string]string{},
		lastScan:     time.Now(),
	}
	handler := buildSessionsHandler(wc)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/sessions", nil))

	var resp apiSessionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(resp.Sessions))
	}
	if got := resp.Sessions[0].SessionID; got != "abcdefghijkl" {
		t.Errorf("session_id = %q, want %q (truncated to 12)", got, "abcdefghijkl")
	}
}

func TestAPISessionsHandler_EmptyCache(t *testing.T) {
	wc := &webCache{
		sessions:     nil,
		sessProjects: nil,
		lastScan:     time.Now(),
	}
	handler := buildSessionsHandler(wc)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/sessions", nil))

	var resp apiSessionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
	if len(resp.Sessions) != 0 {
		t.Errorf("sessions count = %d, want 0", len(resp.Sessions))
	}
}

// --- writeJSON ---

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, map[string]string{"key": "value"})

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["key"] != "value" {
		t.Errorf("key = %q, want %q", resp["key"], "value")
	}
}
