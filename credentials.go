package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
