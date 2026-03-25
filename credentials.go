package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// credentialStatus describes the state of OAuth credentials in a config dir.
type credentialStatus int

const (
	credentialOK      credentialStatus = iota
	credentialMissing                  // no .credentials.json or unreadable
	credentialNoToken                  // file exists but no access token
	credentialExpired                  // access token expired without refresh token
)

// checkCredentials inspects the OAuth credentials in the given config directory.
// Returns credentialOK if credentials appear valid locally. Note: this cannot
// detect server-side token revocation — use "cx login" for that case.
func checkCredentials(configDir string) credentialStatus {
	credPath := filepath.Join(configDir, ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return credentialMissing
	}

	var creds struct {
		ClaudeAiOauth struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresAt    int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return credentialMissing
	}

	if creds.ClaudeAiOauth.AccessToken == "" {
		return credentialNoToken
	}

	// Expired with no refresh token means login is definitely required.
	if creds.ClaudeAiOauth.ExpiresAt > 0 &&
		creds.ClaudeAiOauth.ExpiresAt < time.Now().UnixMilli() &&
		creds.ClaudeAiOauth.RefreshToken == "" {
		return credentialExpired
	}

	return credentialOK
}

// accountInfo consolidates all identity and subscription data readable
// from a config directory's local files (.credentials.json + .claude.json).
// This avoids redundant file reads when multiple fields are needed.
type accountInfo struct {
	// From .credentials.json
	Tier string // "Max 20x", "Max 5x", "Pro", etc.

	// From .claude.json oauthAccount
	Email       string
	DisplayName string
	OrgName     string
	AccountUUID string

	// From .claude.json top-level
	CCVersion   string // last CC version used with this account
	NumStartups int    // total CC sessions started
}

// readAccountInfo reads identity and subscription data from a config directory.
func readAccountInfo(configDir string) *accountInfo {
	info := &accountInfo{}

	// Read tier from .credentials.json.
	if data, err := os.ReadFile(filepath.Join(configDir, ".credentials.json")); err == nil {
		var creds struct {
			ClaudeAiOauth struct {
				SubscriptionType string `json:"subscriptionType"`
				RateLimitTier    string `json:"rateLimitTier"`
			} `json:"claudeAiOauth"`
		}
		if json.Unmarshal(data, &creds) == nil {
			info.Tier = parseTier(creds.ClaudeAiOauth.RateLimitTier, creds.ClaudeAiOauth.SubscriptionType)
		}
	}

	// Read identity + stats from .claude.json.
	if data, err := os.ReadFile(filepath.Join(configDir, ".claude.json")); err == nil {
		var raw struct {
			OAuthAccount struct {
				EmailAddress     string `json:"emailAddress"`
				DisplayName      string `json:"displayName"`
				OrganizationName string `json:"organizationName"`
				AccountUUID      string `json:"accountUuid"`
			} `json:"oauthAccount"`
			LastReleaseNotesSeen string `json:"lastReleaseNotesSeen"`
			NumStartups          int    `json:"numStartups"`
		}
		if json.Unmarshal(data, &raw) == nil {
			info.Email = raw.OAuthAccount.EmailAddress
			info.DisplayName = raw.OAuthAccount.DisplayName
			info.OrgName = raw.OAuthAccount.OrganizationName
			info.AccountUUID = raw.OAuthAccount.AccountUUID
			info.CCVersion = raw.LastReleaseNotesSeen
			info.NumStartups = raw.NumStartups
		}
	}

	return info
}

// parseTier converts a raw rateLimitTier string to a human-readable label.
func parseTier(rateLimitTier, subscriptionType string) string {
	tier := strings.ToLower(rateLimitTier)
	switch {
	case tier == "" && subscriptionType != "":
		return subscriptionType
	case strings.Contains(tier, "20x"):
		return "Max 20x"
	case strings.Contains(tier, "5x"):
		return "Max 5x"
	case strings.Contains(tier, "max"):
		return "Max"
	case strings.Contains(tier, "pro"):
		return "Pro"
	case tier != "":
		return rateLimitTier
	default:
		return ""
	}
}

// credentialMessage returns a human-readable description of the status.
func credentialMessage(status credentialStatus) string {
	switch status {
	case credentialMissing:
		return "No credentials found"
	case credentialNoToken:
		return "Credentials incomplete"
	case credentialExpired:
		return "Credentials expired"
	default:
		return ""
	}
}
