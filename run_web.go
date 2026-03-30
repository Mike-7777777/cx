package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/usage"
)

// webCmd implements Runner for the "web" subcommand.
type webCmd struct{}

const webStaleThreshold = 10 * time.Minute

//go:embed web/index.html
var webFS embed.FS

// webCache holds pre-computed API responses refreshed in the background.
type webCache struct {
	mu       sync.RWMutex
	daily    []usage.DailyReport
	sessions []usage.SessionReport
	// sessEntries keeps the 24h entries used to resolve project names.
	sessEntries []usage.Entry
	roi         apiROIResponse
	lastScan    time.Time
}

func (wc *webCache) refresh(configDirs []string) {
	since := time.Now().UTC().AddDate(0, 0, -30)
	var recent []usage.Entry
	var totalCost float64

	// Use incremental cache for fast warm loads (~0.2s vs ~8s full scan).
	cachePath := usageCachePath()
	uc, _ := usage.LoadUsageCache(cachePath)

	for _, dir := range configDirs {
		_ = usage.ScanDirCached(dir, uc, func(e usage.Entry) {
			totalCost += usage.CalculateCost(e.Model, e.Usage)
			if !e.Timestamp.Before(since) {
				recent = append(recent, e)
			}
		})
	}

	// Persist cache for next refresh.
	_ = uc.Save()

	daily := usage.AggregateDailies(recent)

	// Count distinct months for ROI context.
	monthSet := make(map[string]bool)
	for _, d := range daily {
		if len(d.Date) >= 7 {
			monthSet[d.Date[:7]] = true
		}
	}

	// Sessions: last 24h.
	sessionSince := time.Now().UTC().Add(-24 * time.Hour)
	var sessionEntries []usage.Entry
	for _, e := range recent {
		if !e.Timestamp.Before(sessionSince) {
			sessionEntries = append(sessionEntries, e)
		}
	}
	sessions := usage.AggregateSessions(sessionEntries)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})
	// ROI across all history.
	subCost := totalSubscriptionCost()
	savings := totalCost - subCost
	var pct float64
	if totalCost > 0 {
		pct = savings / totalCost * 100
	}

	wc.mu.Lock()
	wc.daily = daily
	wc.sessions = sessions
	wc.sessEntries = sessionEntries
	wc.roi = apiROIResponse{
		SubscriptionCost:  subCost,
		EquivalentAPICost: totalCost,
		Savings:           savings,
		SavingsPct:        pct,
		Months:            len(monthSet),
		AccountCount:      len(configDirs),
	}
	wc.lastScan = time.Now()
	wc.mu.Unlock()
}

// Run starts the browser dashboard HTTP server.
func (c *webCmd) Run(ctx context.Context, app *App, args []string) error {
	w := app.Stderr
	port := 8099
	noOpen := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 >= len(args) {
				return fmt.Errorf("--port requires a value")
			}
			i++
			n := 0
			for _, ch := range args[i] {
				if ch < '0' || ch > '9' {
					return fmt.Errorf("invalid port %q", args[i])
				}
				n = n*10 + int(ch-'0')
			}
			port = n
		case "--no-open":
			noOpen = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	// Load registry once at server start (it doesn't change during a web session).
	reg, configDirs, err := loadWebRegistry()
	if err != nil {
		return err
	}

	// Create empty cache; initial load + periodic refresh happen async.
	// The HTTP server starts immediately so the browser doesn't wait.
	wc := &webCache{}

	go func() {
		t0 := time.Now()
		wc.refresh(configDirs)
		fmt.Fprintf(w, "Data loaded in %s\n", time.Since(t0).Round(time.Millisecond))

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				wc.refresh(configDirs)
			}
		}
	}()

	mux := http.NewServeMux()

	// Serve embedded HTML.
	mux.HandleFunc("/", func(hw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(hw, r)
			return
		}
		data, err := webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(hw, "internal error", http.StatusInternalServerError)
			return
		}
		hw.Header().Set("Content-Type", "text/html; charset=utf-8")
		hw.Write(data) // Best effort; client may have disconnected.
	})

	// API endpoints.
	mux.HandleFunc("/api/status", func(hw http.ResponseWriter, r *http.Request) {
		handleAPIStatus(hw, r, reg)
	})
	mux.HandleFunc("/api/daily", func(hw http.ResponseWriter, r *http.Request) {
		wc.mu.RLock()
		data := wc.daily
		wc.mu.RUnlock()
		writeJSON(hw, data)
	})
	mux.HandleFunc("/api/sessions", func(hw http.ResponseWriter, r *http.Request) {
		wc.mu.RLock()
		sessions := wc.sessions
		entries := wc.sessEntries
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
				Project:     sessionProject(entries, sr.SessionID),
				StartTime:   sr.StartTime.Format(time.RFC3339),
				TotalTokens: sr.Summary.TotalTokens,
				CostUSD:     sr.Summary.CostUSD,
			})
		}
		writeJSON(hw, resp)
	})
	mux.HandleFunc("/api/roi", func(hw http.ResponseWriter, r *http.Request) {
		wc.mu.RLock()
		data := wc.roi
		wc.mu.RUnlock()
		writeJSON(hw, data)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	if !noOpen {
		openBrowser(url)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start the server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	fmt.Fprintf(w, "Dashboard: %s (Ctrl+C to stop)\n", url)

	select {
	case <-ctx.Done():
		fmt.Fprintln(w, "shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(w, "shutdown error: %v\n", err)
		}
		cancel()
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	return nil
}

// openBrowser launches the default browser for the given URL.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// writeJSON marshals v as JSON and writes it to w with appropriate headers.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"marshal failed"}`, http.StatusInternalServerError)
		return
	}
	w.Write(data) // Best effort; client may have disconnected.
}

// --- API Types ---

type apiAccountStatus struct {
	Name        string   `json:"name"`
	FiveHourPct *float64 `json:"five_hour_pct"`
	FiveHourRst string   `json:"five_hour_reset,omitempty"`
	SevenDayPct *float64 `json:"seven_day_pct"`
	Note        string   `json:"note,omitempty"`
}

type apiStatusResponse struct {
	Accounts []apiAccountStatus `json:"accounts"`
}

type apiROIResponse struct {
	SubscriptionCost  float64 `json:"subscription_cost"`
	EquivalentAPICost float64 `json:"equivalent_api_cost"`
	Savings           float64 `json:"savings"`
	SavingsPct        float64 `json:"savings_pct"`
	Months            int     `json:"months"`
	AccountCount      int     `json:"account_count"`
}

type apiSession struct {
	SessionID   string  `json:"session_id"`
	Project     string  `json:"project"`
	StartTime   string  `json:"start_time"`
	TotalTokens int64   `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

type apiSessionsResponse struct {
	Sessions []apiSession `json:"sessions"`
	Total    int          `json:"total"`
	HasMore  bool         `json:"has_more"`
}

// --- API Handlers ---

func handleAPIStatus(w http.ResponseWriter, _ *http.Request, reg *config.Registry) {
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)

	resp := apiStatusResponse{Accounts: make([]apiAccountStatus, 0, len(names))}

	for _, name := range names {
		acc := apiAccountStatus{Name: name}
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			acc.Note = format.LabelNoData
			resp.Accounts = append(resp.Accounts, acc)
			continue
		}

		rc, err := cache.ReadRateCache(filepath.Join(dir, "rate-cache.json"))
		if err != nil || rc == nil || rc.RateLimits == nil {
			acc.Note = format.LabelNoData
			resp.Accounts = append(resp.Accounts, acc)
			continue
		}

		age := rc.Age()
		if age > webStaleThreshold {
			mins := int(age.Minutes())
			acc.Note = fmt.Sprintf("%s %dm", format.LabelStale, mins)
		}

		rl := rc.RateLimits
		if rl.FiveHour != nil {
			if rl.FiveHour.IsReset() {
				zero := 0.0
				acc.FiveHourPct = &zero
				acc.FiveHourRst = format.LabelReset
			} else {
				acc.FiveHourPct = &rl.FiveHour.UsedPercentage
				acc.FiveHourRst = format.FormatDuration(rl.FiveHour.TimeToReset())
			}
		} else {
			acc.Note = format.LabelNoData
		}

		if rl.SevenDay != nil {
			acc.SevenDayPct = &rl.SevenDay.UsedPercentage
		}

		resp.Accounts = append(resp.Accounts, acc)
	}

	writeJSON(w, resp)
}

// --- Helpers ---

// loadWebRegistry loads the registry and resolves config directories once.
// Returns the registry (for status endpoint) and the list of config dirs
// (for usage endpoints).
func loadWebRegistry() (*config.Registry, []string, error) {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil, nil, err
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil, nil, err
	}

	if len(reg.Accounts) == 0 {
		// Fall back to detecting the single default config dir.
		dir, err := config.DetectConfigDir()
		if err != nil {
			return nil, nil, err
		}
		return reg, []string{dir}, nil
	}

	dirs := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
		}
		dirs = append(dirs, dir)
	}
	return reg, dirs, nil
}

// sessionProject extracts the most common ProjectPath for a given session ID.
func sessionProject(entries []usage.Entry, sessionID string) string {
	counts := make(map[string]int)
	for _, e := range entries {
		if e.SessionID == sessionID && e.ProjectPath != "" {
			// Decode the directory name for display.
			counts[decodeProjectName(e.ProjectPath)]++
		}
	}

	if len(counts) == 0 {
		return "--"
	}

	var best string
	var bestCount int
	for proj, cnt := range counts {
		if cnt > bestCount {
			best = proj
			bestCount = cnt
		}
	}
	return best
}

// decodeProjectName converts encoded directory names back to readable form.
// E.g. "I--google_drive-homebase" -> "homebase"
func decodeProjectName(encoded string) string {
	// Take the last segment after the final "-" that isn't a drive letter prefix.
	parts := strings.Split(encoded, "-")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return encoded
}
