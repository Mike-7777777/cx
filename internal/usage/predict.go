package usage

import "time"

const windowSize = 5 * time.Hour

// ExhaustionEstimate holds the prediction result for a single rate window.
type ExhaustionEstimate struct {
	CurrentPct    float64       `json:"CurrentPct"`
	TimeToReset   time.Duration `json:"TimeToReset"`
	VelocityPctH  float64       `json:"VelocityPctPerHour"`
	TimeToExhaust time.Duration `json:"TimeToExhaust"` // 0 means never (within window)
	Exhausted     bool          `json:"Exhausted"`
	ExhaustAt     time.Time     `json:"ExhaustAt,omitempty"`
}

// EstimateExhaustion predicts when the rate window will be exhausted.
// currentPct: current usage percentage (0-100).
// timeToReset: how long until the window resets.
// now: current time.
//
// Logic: velocity = currentPct / elapsed, where elapsed = windowSize - timeToReset.
// remaining = (100 - currentPct) / velocity.
// If exhaustion would happen after reset, TimeToExhaust = 0 (never).
func EstimateExhaustion(currentPct float64, timeToReset time.Duration, now time.Time) ExhaustionEstimate {
	est := ExhaustionEstimate{
		CurrentPct:  currentPct,
		TimeToReset: timeToReset,
	}

	if currentPct >= 100 {
		est.Exhausted = true
		return est
	}

	if currentPct <= 0 {
		// No usage recorded; cannot compute velocity.
		return est
	}

	elapsed := windowSize - timeToReset
	if elapsed <= 0 {
		// Window just started; not enough data to compute velocity.
		return est
	}

	velocityPctH := currentPct / elapsed.Hours()
	est.VelocityPctH = velocityPctH

	remaining := 100.0 - currentPct
	hoursToExhaust := remaining / velocityPctH
	timeToExhaust := time.Duration(float64(time.Hour) * hoursToExhaust)

	// If exhaustion would happen after the window resets, it won't exhaust.
	if timeToExhaust > timeToReset {
		return est
	}

	est.TimeToExhaust = timeToExhaust
	est.ExhaustAt = now.Add(timeToExhaust)
	return est
}

// Velocity holds usage rate metrics over a time window.
type Velocity struct {
	MsgsPerHour   float64 `json:"MsgsPerHour"`
	TokensPerHour float64 `json:"TokensPerHour"`
	CostPerHour   float64 `json:"CostPerHour"`
}

// CalculateVelocity computes the average usage rate from entries within the window.
// Only entries newer than (now - window) are counted.
func CalculateVelocity(entries []Entry, window time.Duration) Velocity {
	cutoff := time.Now().Add(-window)

	var msgs int
	var tokens int64
	var cost float64

	for _, e := range entries {
		if e.Timestamp.Before(cutoff) {
			continue
		}
		msgs++
		tokens += e.Usage.InputTokens + e.Usage.OutputTokens +
			e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
		cost += CalculateCost(e.Model, e.Usage)
	}

	hours := window.Hours()
	if hours <= 0 || msgs == 0 {
		return Velocity{}
	}

	return Velocity{
		MsgsPerHour:   float64(msgs) / hours,
		TokensPerHour: float64(tokens) / hours,
		CostPerHour:   cost / hours,
	}
}
