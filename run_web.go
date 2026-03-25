package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/usage"
)

//go:embed web/index.html
var webFS embed.FS

func runWeb() {
	port := 8099
	noOpen := false

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cx web: --port requires a value")
				os.Exit(1)
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					fmt.Fprintf(os.Stderr, "cx web: invalid port %q\n", args[i])
					os.Exit(1)
				}
				n = n*10 + int(c-'0')
			}
			port = n
		case "--no-open":
			noOpen = true
		default:
			fmt.Fprintf(os.Stderr, "cx web: unknown flag %q\n", args[i])
			os.Exit(1)
		}
	}

	mux := http.NewServeMux()

	// Serve embedded HTML.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// API endpoints.
	mux.HandleFunc("/api/status", handleAPIStatus)
	mux.HandleFunc("/api/daily", handleAPIDaily)
	mux.HandleFunc("/api/weekly", handleAPIWeekly)
	mux.HandleFunc("/api/monthly", handleAPIMonthly)
	mux.HandleFunc("/api/sessions", handleAPISessions)
	mux.HandleFunc("/api/roi", handleAPIROI)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	if !noOpen {
		openBrowser(url)
	}

	fmt.Fprintf(os.Stderr, "Dashboard: %s (Ctrl+C to stop)\n", url)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "cx web: %v\n", err)
		os.Exit(1)
	}
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
	w.Write(data)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- API Types ---

type apiAccountStatus struct {
	Name         string   `json:"name"`
	FiveHourPct  *float64 `json:"five_hour_pct"`
	FiveHourRst  string   `json:"five_hour_reset,omitempty"`
	SevenDayPct  *float64 `json:"seven_day_pct"`
	Note         string   `json:"note,omitempty"`
}

type apiStatusResponse struct {
	Accounts []apiAccountStatus `json:"accounts"`
}

type apiROIResponse struct {
	SubscriptionCost  float64 `json:"subscription_cost"`
	EquivalentAPICost float64 `json:"equivalent_api_cost"`
	Savings           float64 `json:"savings"`
	SavingsPct        float64 `json:"savings_pct"`
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
}

// --- API Handlers ---

func handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	regPath, err := config.RegistryPath()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
			acc.Note = "no data"
			resp.Accounts = append(resp.Accounts, acc)
			continue
		}

		rc, err := cache.ReadRateCache(filepath.Join(dir, "rate-cache.json"))
		if err != nil || rc == nil || rc.RateLimits == nil {
			acc.Note = "no data"
			resp.Accounts = append(resp.Accounts, acc)
			continue
		}

		age := rc.Age()
		if age > 10*time.Minute {
			mins := int(age.Minutes())
			acc.Note = fmt.Sprintf("stale %dm", mins)
		}

		rl := rc.RateLimits
		if rl.FiveHour != nil {
			if rl.FiveHour.IsReset() {
				zero := 0.0
				acc.FiveHourPct = &zero
				acc.FiveHourRst = "reset"
			} else {
				acc.FiveHourPct = &rl.FiveHour.UsedPercentage
				ttr := rl.FiveHour.TimeToReset()
				h := int(ttr.Hours())
				m := int(ttr.Minutes()) % 60
				if h > 0 {
					acc.FiveHourRst = fmt.Sprintf("%dh%dm", h, m)
				} else {
					acc.FiveHourRst = fmt.Sprintf("%dm", m)
				}
			}
		} else {
			acc.Note = "no data"
		}

		if rl.SevenDay != nil {
			acc.SevenDayPct = &rl.SevenDay.UsedPercentage
		}

		resp.Accounts = append(resp.Accounts, acc)
	}

	writeJSON(w, resp)
}

func handleAPIDaily(w http.ResponseWriter, r *http.Request) {
	configDirs, err := webConfigDirs()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Scan entries for the last 30 days.
	since := time.Now().UTC().AddDate(0, 0, -30)
	var entries []usage.Entry
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			if !e.Timestamp.Before(since) {
				entries = append(entries, e)
			}
		})
	}

	reports := usage.AggregateDailies(entries)
	writeJSON(w, reports)
}

func handleAPIWeekly(w http.ResponseWriter, r *http.Request) {
	configDirs, err := webConfigDirs()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var entries []usage.Entry
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			entries = append(entries, e)
		})
	}

	reports := usage.AggregateWeekly(entries)
	writeJSON(w, reports)
}

func handleAPIMonthly(w http.ResponseWriter, r *http.Request) {
	configDirs, err := webConfigDirs()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var entries []usage.Entry
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			entries = append(entries, e)
		})
	}

	reports := usage.AggregateMonthly(entries)
	writeJSON(w, reports)
}

func handleAPISessions(w http.ResponseWriter, r *http.Request) {
	configDirs, err := webConfigDirs()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Only show sessions from the last 24 hours for "active" sessions.
	since := time.Now().UTC().Add(-24 * time.Hour)
	var entries []usage.Entry
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			if !e.Timestamp.Before(since) {
				entries = append(entries, e)
			}
		})
	}

	sessionReports := usage.AggregateSessions(entries)

	// Sort by start time descending (most recent first), limit to 20.
	sort.Slice(sessionReports, func(i, j int) bool {
		return sessionReports[i].StartTime.After(sessionReports[j].StartTime)
	})
	if len(sessionReports) > 20 {
		sessionReports = sessionReports[:20]
	}

	resp := apiSessionsResponse{Sessions: make([]apiSession, 0, len(sessionReports))}
	for _, sr := range sessionReports {
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

	writeJSON(w, resp)
}

func handleAPIROI(w http.ResponseWriter, r *http.Request) {
	configDirs, err := webConfigDirs()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Scan all entries to calculate total API-equivalent cost.
	var entries []usage.Entry
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			entries = append(entries, e)
		})
	}

	var totalCost float64
	for _, e := range entries {
		totalCost += usage.CalculateCost(e.Model, e.Usage)
	}

	subCost := totalSubscriptionCost()
	savings := totalCost - subCost
	var pct float64
	if totalCost > 0 {
		pct = savings / totalCost * 100
	}

	writeJSON(w, apiROIResponse{
		SubscriptionCost:  subCost,
		EquivalentAPICost: totalCost,
		Savings:           savings,
		SavingsPct:        pct,
	})
}

// --- Helpers ---

// webConfigDirs returns all registered account config directories.
func webConfigDirs() ([]string, error) {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil, err
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil, err
	}

	if len(reg.Accounts) == 0 {
		// Fall back to detecting the single default config dir.
		dir, err := config.DetectConfigDir()
		if err != nil {
			return nil, err
		}
		return []string{dir}, nil
	}

	dirs := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
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
