package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parseTier
// ---------------------------------------------------------------------------

func TestParseTier(t *testing.T) {
	tests := []struct {
		name             string
		rateLimitTier    string
		subscriptionType string
		want             string
	}{
		// Exact matches for each branch
		{name: "20x tier", rateLimitTier: "claude_max_20x", subscriptionType: "", want: "Max 20x"},
		{name: "5x tier", rateLimitTier: "claude_max_5x", subscriptionType: "", want: "Max 5x"},
		{name: "max tier", rateLimitTier: "claude_max", subscriptionType: "", want: "Max"},
		{name: "pro tier", rateLimitTier: "claude_pro", subscriptionType: "", want: "Pro"},

		// Case insensitivity
		{name: "20x upper", rateLimitTier: "CLAUDE_MAX_20X", subscriptionType: "", want: "Max 20x"},
		{name: "pro mixed case", rateLimitTier: "Claude_Pro", subscriptionType: "", want: "Pro"},

		// Fallback to subscriptionType when rateLimitTier is empty
		{name: "empty tier with sub", rateLimitTier: "", subscriptionType: "enterprise", want: "enterprise"},

		// Unknown non-empty tier returns raw value
		{name: "unknown tier", rateLimitTier: "beta_access", subscriptionType: "", want: "beta_access"},

		// Both empty
		{name: "both empty", rateLimitTier: "", subscriptionType: "", want: ""},

		// Priority: tier containing "max" wins over "pro" substring
		{name: "max wins over pro substring", rateLimitTier: "max_pro_thing", subscriptionType: "", want: "Max"},

		// 20x takes priority over plain max
		{name: "20x beats max", rateLimitTier: "max_20x", subscriptionType: "", want: "Max 20x"},

		// 5x takes priority over plain max
		{name: "5x beats max", rateLimitTier: "max_5x", subscriptionType: "", want: "Max 5x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTier(tt.rateLimitTier, tt.subscriptionType)
			if got != tt.want {
				t.Errorf("parseTier(%q, %q) = %q, want %q",
					tt.rateLimitTier, tt.subscriptionType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// readAccountInfo
// ---------------------------------------------------------------------------

func TestReadAccountInfo_FullData(t *testing.T) {
	dir := t.TempDir()

	credJSON := `{
		"claudeAiOauth": {
			"subscriptionType": "pro_plan",
			"rateLimitTier":    "claude_pro"
		}
	}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(credJSON), 0o600)

	claudeJSON := `{
		"oauthAccount": {
			"emailAddress":     "user@example.com",
			"displayName":      "Test User",
			"organizationName": "Acme",
			"accountUuid":      "abc-123"
		},
		"lastReleaseNotesSeen": "1.0.42",
		"numStartups":          7
	}`
	os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(claudeJSON), 0o600)

	info := readAccountInfo(dir)

	if info.Tier != "Pro" {
		t.Errorf("Tier = %q, want %q", info.Tier, "Pro")
	}
	if info.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", info.Email, "user@example.com")
	}
	if info.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", info.DisplayName, "Test User")
	}
	if info.OrgName != "Acme" {
		t.Errorf("OrgName = %q, want %q", info.OrgName, "Acme")
	}
	if info.AccountUUID != "abc-123" {
		t.Errorf("AccountUUID = %q, want %q", info.AccountUUID, "abc-123")
	}
	if info.CCVersion != "1.0.42" {
		t.Errorf("CCVersion = %q, want %q", info.CCVersion, "1.0.42")
	}
	if info.NumStartups != 7 {
		t.Errorf("NumStartups = %d, want %d", info.NumStartups, 7)
	}
}

func TestReadAccountInfo_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	info := readAccountInfo(dir)

	if info.Email != "" {
		t.Errorf("Email should be empty, got %q", info.Email)
	}
	if info.Tier != "" {
		t.Errorf("Tier should be empty, got %q", info.Tier)
	}
	if info.NumStartups != 0 {
		t.Errorf("NumStartups should be 0, got %d", info.NumStartups)
	}
}

func TestReadAccountInfo_CredentialsOnly(t *testing.T) {
	dir := t.TempDir()

	credJSON := `{"claudeAiOauth":{"rateLimitTier":"claude_max_5x"}}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(credJSON), 0o600)

	info := readAccountInfo(dir)

	if info.Tier != "Max 5x" {
		t.Errorf("Tier = %q, want %q", info.Tier, "Max 5x")
	}
	if info.Email != "" {
		t.Errorf("Email should be empty without .claude.json, got %q", info.Email)
	}
}

func TestReadAccountInfo_ClaudeJSONOnly(t *testing.T) {
	dir := t.TempDir()

	claudeJSON := `{"oauthAccount":{"emailAddress":"solo@test.com"},"numStartups":3}`
	os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(claudeJSON), 0o600)

	info := readAccountInfo(dir)

	if info.Tier != "" {
		t.Errorf("Tier should be empty without .credentials.json, got %q", info.Tier)
	}
	if info.Email != "solo@test.com" {
		t.Errorf("Email = %q, want %q", info.Email, "solo@test.com")
	}
	if info.NumStartups != 3 {
		t.Errorf("NumStartups = %d, want %d", info.NumStartups, 3)
	}
}

func TestReadAccountInfo_MalformedJSON(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`{bad`), 0o600)
	os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(`{bad`), 0o600)

	info := readAccountInfo(dir)

	// Should not crash; all fields stay zero-valued.
	if info.Tier != "" {
		t.Errorf("Tier should be empty for malformed JSON, got %q", info.Tier)
	}
	if info.Email != "" {
		t.Errorf("Email should be empty for malformed JSON, got %q", info.Email)
	}
}

// ---------------------------------------------------------------------------
// checkCredentials
// ---------------------------------------------------------------------------

func TestCheckCredentials_NoFile(t *testing.T) {
	dir := t.TempDir()
	status := checkCredentials(dir)
	if status != credentialMissing {
		t.Errorf("status = %d, want credentialMissing (%d)", status, credentialMissing)
	}
}

func TestCheckCredentials_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`not json`), 0o600)

	status := checkCredentials(dir)
	if status != credentialMissing {
		t.Errorf("status = %d, want credentialMissing (%d)", status, credentialMissing)
	}
}

func TestCheckCredentials_NoToken(t *testing.T) {
	dir := t.TempDir()
	cred := `{"claudeAiOauth":{}}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(cred), 0o600)

	status := checkCredentials(dir)
	if status != credentialNoToken {
		t.Errorf("status = %d, want credentialNoToken (%d)", status, credentialNoToken)
	}
}

func TestCheckCredentials_EmptyAccessToken(t *testing.T) {
	dir := t.TempDir()
	cred := `{"claudeAiOauth":{"accessToken":""}}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(cred), 0o600)

	status := checkCredentials(dir)
	if status != credentialNoToken {
		t.Errorf("status = %d, want credentialNoToken (%d)", status, credentialNoToken)
	}
}

func TestCheckCredentials_ValidToken(t *testing.T) {
	dir := t.TempDir()
	futureMs := time.Now().Add(time.Hour).UnixMilli()
	cred := `{"claudeAiOauth":{"accessToken":"tok","expiresAt":` +
		itoa64(futureMs) + `}}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(cred), 0o600)

	status := checkCredentials(dir)
	if status != credentialOK {
		t.Errorf("status = %d, want credentialOK (%d)", status, credentialOK)
	}
}

func TestCheckCredentials_ValidTokenNoExpiry(t *testing.T) {
	dir := t.TempDir()
	// expiresAt omitted (zero value) — treated as valid.
	cred := `{"claudeAiOauth":{"accessToken":"tok"}}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(cred), 0o600)

	status := checkCredentials(dir)
	if status != credentialOK {
		t.Errorf("status = %d, want credentialOK (%d)", status, credentialOK)
	}
}

func TestCheckCredentials_ExpiredNoRefresh(t *testing.T) {
	dir := t.TempDir()
	pastMs := time.Now().Add(-time.Hour).UnixMilli()
	cred := `{"claudeAiOauth":{"accessToken":"tok","expiresAt":` +
		itoa64(pastMs) + `}}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(cred), 0o600)

	status := checkCredentials(dir)
	if status != credentialExpired {
		t.Errorf("status = %d, want credentialExpired (%d)", status, credentialExpired)
	}
}

func TestCheckCredentials_ExpiredWithRefresh(t *testing.T) {
	dir := t.TempDir()
	pastMs := time.Now().Add(-time.Hour).UnixMilli()
	cred := `{"claudeAiOauth":{"accessToken":"tok","expiresAt":` +
		itoa64(pastMs) + `,"refreshToken":"rt"}}`
	os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(cred), 0o600)

	// Has a refresh token, so should be OK despite expiry.
	status := checkCredentials(dir)
	if status != credentialOK {
		t.Errorf("status = %d, want credentialOK (%d)", status, credentialOK)
	}
}

// ---------------------------------------------------------------------------
// credentialMessage
// ---------------------------------------------------------------------------

func TestCredentialMessage(t *testing.T) {
	tests := []struct {
		status  credentialStatus
		wantSub string // expected substring; "" means exact empty
	}{
		{credentialOK, ""},
		{credentialMissing, "No credentials"},
		{credentialNoToken, "incomplete"},
		{credentialExpired, "expired"},
	}
	for _, tt := range tests {
		msg := credentialMessage(tt.status)
		if tt.status == credentialOK {
			if msg != "" {
				t.Errorf("credentialMessage(credentialOK) = %q, want empty", msg)
			}
			continue
		}
		if msg == "" {
			t.Errorf("credentialMessage(%d) returned empty, want substring %q", tt.status, tt.wantSub)
		}
		if tt.wantSub != "" && !strings.Contains(msg, tt.wantSub) {
			t.Errorf("credentialMessage(%d) = %q, missing substring %q", tt.status, msg, tt.wantSub)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// itoa64 formats an int64 as a decimal string for embedding in JSON literals.
func itoa64(n int64) string {
	return fmt.Sprintf("%d", n)
}
