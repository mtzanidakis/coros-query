package coros

import (
	"math"
	"testing"
)

func approx(a, b, eps float64) bool { return math.Abs(a-b) < eps }

func TestComputeDerived_FromRealSample(t *testing.T) {
	// Mirrors the live 2026-04-11 sample. Target day matches the last entry.
	rng := []Day{
		{HappenDay: 20260405, TrainingLoad: 61, T7D: 645, T28D: 2497, RHR: 50},
		{HappenDay: 20260406, TrainingLoad: 111, T7D: 643, T28D: 2470, RHR: 51},
		{HappenDay: 20260407, TrainingLoad: 57, T7D: 541, T28D: 2330, RHR: 54},
		{HappenDay: 20260408, TrainingLoad: 81, T7D: 536, T28D: 2400, RHR: 54},
		{HappenDay: 20260409, TrainingLoad: 62, T7D: 528, T28D: 2377, RHR: 51},
		{HappenDay: 20260410, TrainingLoad: 0, T7D: 507, T28D: 2213, RHR: 58},
		{
			HappenDay: 20260411, TrainingLoad: 0, T7D: 372, T28D: 2142,
			RHR: 51, AvgSleepHrv: 25, SleepHrvBase: 26,
			SleepHrvIntervalList:   []int{5, 19, 22, 30},
			Ct7dMin:                358.4,
			TiredRateStateNew:      3,
			TrainingLoadRatioState: 3,
		},
	}
	d := ComputeDerived(rng, &rng[len(rng)-1])

	if d.HrvZScore == nil || !approx(*d.HrvZScore, -0.25, 0.01) {
		t.Errorf("hrv_zscore = %v, want ≈ -0.25", d.HrvZScore)
	}
	// Acute:chronic = 372 / (2142/4) = 372/535.5 ≈ 0.69
	if d.AcuteChronicRatio == nil || !approx(*d.AcuteChronicRatio, 0.69, 0.01) {
		t.Errorf("acute_chronic_ratio = %v, want ≈ 0.69", d.AcuteChronicRatio)
	}
	// RHR delta: 51 - mean(50,51,54,54,51,58) = 51 - 53 = -2
	if d.RhrDelta7d == nil || !approx(*d.RhrDelta7d, -2, 0.4) {
		t.Errorf("rhr_delta_7d = %v, want ≈ -2", d.RhrDelta7d)
	}
	// t7d slope should be negative (strongly decreasing over the window)
	if d.TlTrend7dSlope == nil || *d.TlTrend7dSlope >= 0 {
		t.Errorf("tl_trend_7d_slope = %v, want negative", d.TlTrend7dSlope)
	}
	if d.Monotony7d == nil || *d.Monotony7d <= 0 {
		t.Errorf("monotony = %v", d.Monotony7d)
	}
	if d.Strain7d == nil || *d.Strain7d <= 0 {
		t.Errorf("strain = %v", d.Strain7d)
	}
	// Last hard session: TL > 358.4/7 ≈ 51.2. rng[4].TrainingLoad=62 is the
	// last such day (index 4, range len 7), so days_since_hard = 2.
	if d.DaysSinceHard == nil || *d.DaysSinceHard != 2 {
		t.Errorf("days_since_hard = %v, want 2", d.DaysSinceHard)
	}
	// Readiness composite: sanity bounds
	if d.ReadinessScore == nil || *d.ReadinessScore < 0 || *d.ReadinessScore > 100 {
		t.Errorf("readiness = %v", d.ReadinessScore)
	}
}

func TestComputeDerived_EmptyRangeIsSafe(t *testing.T) {
	d := ComputeDerived(nil, nil)
	if d.HrvZScore != nil || d.ReadinessScore != nil {
		t.Errorf("want all-nil, got %+v", d)
	}
}

func TestComputeDerived_MissingHRVSkipsZScore(t *testing.T) {
	rng := []Day{
		{HappenDay: 20260411, RHR: 50, TrainingLoadRatioState: 3, TiredRateStateNew: 2},
	}
	d := ComputeDerived(rng, &rng[0])
	if d.HrvZScore != nil {
		t.Errorf("hrv_zscore should be nil without HRV data, got %v", *d.HrvZScore)
	}
	// Readiness still computed from fatigue + TL ratio zones
	if d.ReadinessScore == nil {
		t.Errorf("readiness should still be computable")
	}
}

func TestZoneToScore_FatigueAndRatio(t *testing.T) {
	if zoneToScore(1, true) != 100 || zoneToScore(5, true) != 15 {
		t.Errorf("fatigue zone scoring wrong")
	}
	if zoneToScore(3, false) != 100 {
		t.Errorf("optimal TL ratio should score 100")
	}
	if zoneToScore(1, false) >= zoneToScore(3, false) {
		t.Errorf("detraining should not beat optimal")
	}
	if zoneToScore(5, false) >= zoneToScore(3, false) {
		t.Errorf("overreach should not beat optimal")
	}
}

func TestZScoreToScore_Monotonic(t *testing.T) {
	// Higher z (healthier HRV) should never map to a lower score.
	prev := -1.0
	for _, z := range []float64{-3, -2, -1.5, -1, -0.5, 0, 0.5, 1} {
		s := zScoreToScore(z)
		if s < prev {
			t.Errorf("zScoreToScore not monotonic at z=%v: %v < %v", z, s, prev)
		}
		prev = s
	}
}

func TestRhrDeltaToScore_LowerIsBetter(t *testing.T) {
	if rhrDeltaToScore(-5) <= rhrDeltaToScore(5) {
		t.Errorf("lower RHR delta should score higher")
	}
	if rhrDeltaToScore(0) <= rhrDeltaToScore(6) {
		t.Errorf("zero delta should beat +6")
	}
}

func TestStatsHelpers(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	if m := mean(xs); !approx(m, 3, 1e-9) {
		t.Errorf("mean = %v", m)
	}
	if s := stddev(xs, 3); !approx(s, math.Sqrt(2.5), 1e-9) {
		t.Errorf("stddev = %v", s)
	}
	if s, ok := slope([]float64{0, 1, 2, 3}, []float64{1, 2, 3, 4}); !ok || !approx(s, 1, 1e-9) {
		t.Errorf("slope = %v ok=%v", s, ok)
	}
	if _, ok := slope([]float64{0}, []float64{1}); ok {
		t.Errorf("slope with 1 point should return false")
	}
}

func TestRound2(t *testing.T) {
	cases := map[float64]float64{
		1.234:  1.23,
		1.235:  1.24,
		-0.666: -0.67,
	}
	for in, want := range cases {
		if got := round2(in); !approx(got, want, 1e-9) {
			t.Errorf("round2(%v) = %v, want %v", in, got, want)
		}
	}
}
