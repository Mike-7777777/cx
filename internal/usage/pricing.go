package usage

import (
	"fmt"
	"os"
	"time"
)

// PricingUpdatedAt is the date the pricing table was last updated.
var PricingUpdatedAt = time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)

// CheckPricingStaleness prints a warning to stderr if the pricing data is
// older than 30 days. Call once at startup, not in hot loops.
func CheckPricingStaleness() {
	age := time.Since(PricingUpdatedAt)
	if age > 30*24*time.Hour {
		days := int(age.Hours() / 24)
		fmt.Fprintf(os.Stderr, "[cx] Warning: pricing table is %d days old (last updated %s). Costs may be inaccurate.\n",
			days, PricingUpdatedAt.Format("2006-01-02"))
	}
}

// ModelPricing holds per-million-token prices for a Claude model.
type ModelPricing struct {
	InputPerMTok       float64 // $ per million input tokens
	OutputPerMTok      float64 // $ per million output tokens
	CacheReadPerMTok   float64 // $ per million cache read tokens
	CacheCreatePerMTok float64 // $ per million cache creation tokens
}

// Pricing maps model ID to its token pricing. Covers all known Claude models
// as of 2026-03. Unknown models fall back to Sonnet pricing in CalculateCost.
var Pricing = map[string]ModelPricing{
	// Claude 4.6
	"claude-opus-4-6":   {InputPerMTok: 15.0, OutputPerMTok: 75.0, CacheReadPerMTok: 1.5, CacheCreatePerMTok: 18.75},
	"claude-sonnet-4-6": {InputPerMTok: 3.0, OutputPerMTok: 15.0, CacheReadPerMTok: 0.3, CacheCreatePerMTok: 3.75},

	// Claude 4.5
	"claude-sonnet-4-5-20250929": {InputPerMTok: 3.0, OutputPerMTok: 15.0, CacheReadPerMTok: 0.3, CacheCreatePerMTok: 3.75},
	"claude-opus-4-5-20251101":   {InputPerMTok: 15.0, OutputPerMTok: 75.0, CacheReadPerMTok: 1.5, CacheCreatePerMTok: 18.75},

	// Claude 4
	"claude-opus-4-20250514":   {InputPerMTok: 15.0, OutputPerMTok: 75.0, CacheReadPerMTok: 1.5, CacheCreatePerMTok: 18.75},
	"claude-sonnet-4-20250514": {InputPerMTok: 3.0, OutputPerMTok: 15.0, CacheReadPerMTok: 0.3, CacheCreatePerMTok: 3.75},

	// Claude 3.5 / 3
	"claude-3-5-sonnet-20241022": {InputPerMTok: 3.0, OutputPerMTok: 15.0, CacheReadPerMTok: 0.3, CacheCreatePerMTok: 3.75},
	"claude-3-5-haiku-20241022":  {InputPerMTok: 0.8, OutputPerMTok: 4.0, CacheReadPerMTok: 0.08, CacheCreatePerMTok: 1.0},

	// Haiku 4.5
	"claude-haiku-4-5-20251001": {InputPerMTok: 0.8, OutputPerMTok: 4.0, CacheReadPerMTok: 0.08, CacheCreatePerMTok: 1.0},
}

// sonnetFallback is the default pricing for unknown models.
var sonnetFallback = Pricing["claude-sonnet-4-6"]

// CalculateCost returns the estimated API cost in USD for the given usage and model.
// If the model is not in the pricing table, Sonnet pricing is used as a safe default.
func CalculateCost(model string, u TokenUsage) float64 {
	p, ok := Pricing[model]
	if !ok {
		p = sonnetFallback
	}
	const m = 1_000_000.0
	return float64(u.InputTokens)/m*p.InputPerMTok +
		float64(u.OutputTokens)/m*p.OutputPerMTok +
		float64(u.CacheCreationInputTokens)/m*p.CacheCreatePerMTok +
		float64(u.CacheReadInputTokens)/m*p.CacheReadPerMTok
}
