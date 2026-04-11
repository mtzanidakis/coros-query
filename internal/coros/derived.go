package coros

import "math"

// Derived holds client-side computed readiness metrics from a Day range
// ending at (and including) the target day. All fields are optional —
// missing inputs yield nil pointers.
type Derived struct {
	HrvZScore         *float64 `json:"hrv_zscore,omitempty"`
	RhrDelta7d        *float64 `json:"rhr_delta_7d,omitempty"`
	TlTrend7dSlope    *float64 `json:"tl_trend_7d_slope,omitempty"`
	AcuteChronicRatio *float64 `json:"acute_chronic_ratio,omitempty"`
	Monotony7d        *float64 `json:"training_monotony_7d,omitempty"`
	Strain7d          *float64 `json:"training_strain_7d,omitempty"`
	DaysSinceHard     *int     `json:"days_since_hard,omitempty"`
	ReadinessScore    *int     `json:"readiness_composite,omitempty"`
}

// ComputeDerived produces Derived from a chronological range and the target
// day (which should be range[len(range)-1] when the caller wants "today").
func ComputeDerived(rng []Day, target *Day) Derived {
	var d Derived
	if target == nil || len(rng) == 0 {
		return d
	}

	// HRV z-score: (today - base) / balanced-band half-width.
	// The UI shows "HRV Base <intervals[2]>-<intervals[3]>" — that range is
	// the personal balanced band, centered on sleepHrvBase.
	if target.AvgSleepHrv > 0 && target.SleepHrvBase > 0 && len(target.SleepHrvIntervalList) >= 4 {
		base := float64(target.SleepHrvBase)
		balLow := float64(target.SleepHrvIntervalList[2])
		balHigh := float64(target.SleepHrvIntervalList[3])
		sigma := (balHigh - balLow) / 2
		if sigma > 0 {
			z := (float64(target.AvgSleepHrv) - base) / sigma
			d.HrvZScore = ptrF(round2(z))
		}
	}

	// RHR delta: today vs mean(prior days in window) — requires >=2 days with rhr>0
	var priorRHR []float64
	for _, x := range rng[:max(len(rng)-1, 0)] {
		if x.RHR > 0 {
			priorRHR = append(priorRHR, float64(x.RHR))
		}
	}
	if target.RHR > 0 && len(priorRHR) > 0 {
		delta := float64(target.RHR) - mean(priorRHR)
		d.RhrDelta7d = ptrF(round2(delta))
	}

	// Training load trend: slope of t7d over the window (linear regression)
	if len(rng) >= 3 {
		xs := make([]float64, len(rng))
		ys := make([]float64, len(rng))
		for i, x := range rng {
			xs[i] = float64(i)
			ys[i] = float64(x.T7D)
		}
		if s, ok := slope(xs, ys); ok {
			d.TlTrend7dSlope = ptrF(round2(s))
		}
	}

	// Acute:chronic ratio from t7d vs t28d/4
	if target.T28D > 0 {
		chronicDaily := float64(target.T28D) / 4.0 // 28d sum scaled to weekly
		if chronicDaily > 0 {
			d.AcuteChronicRatio = ptrF(round2(float64(target.T7D) / chronicDaily))
		}
	}

	// Monotony & strain (Foster) over the TL series in the window
	var tls []float64
	for _, x := range rng {
		tls = append(tls, x.TrainingLoad)
	}
	if len(tls) >= 3 {
		m := mean(tls)
		sd := stddev(tls, m)
		if sd > 0 {
			mono := m / sd
			d.Monotony7d = ptrF(round2(mono))
			strain := sumF(tls) * mono
			d.Strain7d = ptrF(round2(strain))
		}
	}

	// Days since last hard session (TL > ct7dMin/7 as a rough per-day threshold)
	threshold := 0.0
	if target.Ct7dMin > 0 {
		threshold = target.Ct7dMin / 7.0
	}
	if threshold > 0 {
		days := -1
		for i := len(rng) - 1; i >= 0; i-- {
			if rng[i].TrainingLoad >= threshold {
				days = (len(rng) - 1) - i
				break
			}
		}
		if days >= 0 {
			d.DaysSinceHard = ptrI(days)
		}
	}

	// Readiness composite (0..100) — weighted mix of 4 signals
	score := 0.0
	weight := 0.0
	if target.TiredRateStateNew >= 1 {
		score += 0.35 * zoneToScore(target.TiredRateStateNew, true)
		weight += 0.35
	}
	if d.HrvZScore != nil {
		score += 0.25 * zScoreToScore(*d.HrvZScore)
		weight += 0.25
	}
	if d.RhrDelta7d != nil {
		score += 0.15 * rhrDeltaToScore(*d.RhrDelta7d)
		weight += 0.15
	}
	if target.TrainingLoadRatioState >= 1 {
		score += 0.25 * zoneToScore(target.TrainingLoadRatioState, false)
		weight += 0.25
	}
	if weight > 0 {
		r := int(math.Round(score / weight))
		d.ReadinessScore = ptrI(r)
	}

	return d
}

// zoneToScore maps a 1..5 Coros zone to a 0..100 readiness score.
// Fatigue zones: 1=very fresh (best), 5=very fatigued (worst).
// Ratio zones: 3=optimal (best), 1 & 5 are both bad.
func zoneToScore(zone int, fatigue bool) float64 {
	if fatigue {
		switch zone {
		case 1:
			return 100
		case 2:
			return 85
		case 3:
			return 70
		case 4:
			return 40
		case 5:
			return 15
		}
		return 0
	}
	// training load ratio
	switch zone {
	case 1:
		return 60
	case 2:
		return 85
	case 3:
		return 100
	case 4:
		return 60
	case 5:
		return 30
	}
	return 0
}

func zScoreToScore(z float64) float64 {
	switch {
	case z >= 0:
		return 100
	case z >= -1:
		return 60 + (z+1)*40 // -1 → 60, 0 → 100
	case z >= -2:
		return 20 + (z+2)*40 // -2 → 20, -1 → 60
	default:
		return 10
	}
}

func rhrDeltaToScore(delta float64) float64 {
	switch {
	case delta <= -3:
		return 100
	case delta <= 0:
		return 80 + (-delta/3)*20 // 0 → 80, -3 → 100
	case delta <= 3:
		return 50 + (3-delta)/3*30 // 0 → 80, 3 → 50
	case delta <= 6:
		return 20 + (6-delta)/3*30 // 3 → 50, 6 → 20
	default:
		return 10
	}
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	return sumF(xs) / float64(len(xs))
}

func sumF(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s
}

func stddev(xs []float64, m float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var ss float64
	for _, x := range xs {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(xs)-1))
}

func slope(xs, ys []float64) (float64, bool) {
	if len(xs) != len(ys) || len(xs) < 2 {
		return 0, false
	}
	mx := mean(xs)
	my := mean(ys)
	var num, den float64
	for i := range xs {
		num += (xs[i] - mx) * (ys[i] - my)
		den += (xs[i] - mx) * (xs[i] - mx)
	}
	if den == 0 {
		return 0, false
	}
	return num / den, true
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }
func ptrF(x float64) *float64  { return &x }
func ptrI(x int) *int          { return &x }
