package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mtzanidakis/coros-query/internal/coros"
)

const usage = `coros-query — fetch Coros Training Hub daily briefing as JSON.

Usage:
  coros-query [--date YYYY-MM-DD] [--days N] [--region eu|us|cn]

Flags:
  --date YYYY-MM-DD  target date (default: today)
  --days N           trailing window size ending at --date (default: 7)
  --region eu|us|cn  coros region (default: $COROS_REGION or eu)

Environment:
  COROS_EMAIL     (required)
  COROS_PASSWORD  (required)
  COROS_USER_ID   (optional; auto-detected from login if possible)
  COROS_REGION    (optional; default: eu)

Output: JSON on stdout. Sections:
  day         — target day's metrics (training load, fatigue, HRV, RHR)
  range       — trailing N-day window
  weekList    — per-week TL recommendations
  derived     — client-side readiness composite & trends
  activities  — activities within the window
  analytics   — multi-week sport totals & HR zone distribution
  schedule    — active training plan + weekly plan-vs-actual
  errors      — per-section failures (sections may be partial)
  legend      — field glossary
`

var legend = map[string]string{
	"trainingLoad":           "Daily training load (session impulse units)",
	"trainingLoadRatio":      "Acute:chronic training load ratio (ACWR)",
	"trainingLoadRatioState": "Zone for trainingLoadRatio: 1=low, 2=optimal-low, 3=optimal, 4=high, 5=very-high",
	"recomendTlMin":          "Recommended weekly training load floor",
	"recomendTlMax":          "Recommended weekly training load ceiling",
	"tiredRate":              "Legacy fatigue score (0..100)",
	"tiredRateNew":           "Freshness/fatigue delta (negative=fresh, positive=fatigued)",
	"tiredRateStateNew":      "Zone for tiredRateNew: 1=very-fresh..5=very-fatigued",
	"ati":                    "Acute training impact (~7d exponentially weighted load)",
	"cti":                    "Chronic training impact (~28d exponentially weighted load)",
	"tib":                    "Training Impact Balance (cti-ati equivalent; >0 fresh, <0 fatigued)",
	"t7d":                    "Rolling 7-day training load sum",
	"t28d":                   "Rolling 28-day training load sum",
	"ct7dMin":                "Chronic-training 7d floor bound",
	"ct7dMaxFixed":           "Chronic-training 7d ceiling bound",
	"rhr":                    "Resting heart rate for the day (bpm)",
	"testRhr":                "Orthostatic/test RHR measurement (bpm)",
	"avgSleepHrv":            "Average overnight HRV (ms, RMSSD-style)",
	"sleepHrvBase":           "Personal HRV baseline (ms)",
	"sleepHrvIntervalList":   "HRV status band thresholds [low, balanced-low, balanced-high, high]",
	"performance":            "Performance indicator (-1 if not computed)",
	"activities.avgSpeed":    "Cycling: km/h × 100 (when speedType=1). Running: pace (different scale)",
	"activities.calorie":     "kcal × 1000",
	"activities.distance":    "meters",
	"activities.totalTime":   "seconds",
	"sportType":              "100=Run 101=IndoorRun 102=TrailRun 200=RoadBike 201=IndoorBike 300=PoolSwim 65535=All",
	"derived.hrv_zscore":     "(today HRV - base) / balanced-band half-width. <0 = below baseline",
	"derived.readiness_composite": "0..100 weighted mix of fatigue zone, HRV z-score, RHR delta, TL ratio zone",
	"derived.training_monotony_7d": "Foster monotony: mean(TL)/stddev(TL) over window. >2 injury risk",
	"derived.training_strain_7d":   "Foster strain: sum(TL) * monotony over window",
	"derived.acute_chronic_ratio":  "t7d / (t28d/4). 0.8..1.3 sweet spot, >1.5 overreach",
}

func main() {
	var (
		dateStr = flag.String("date", "", "target date YYYY-MM-DD (default: today)")
		days    = flag.Int("days", 7, "trailing window size ending at --date")
		region  = flag.String("region", envOr("COROS_REGION", "eu"), "coros region: eu, us, cn")
	)
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	if *days < 1 {
		fatal("--days must be >= 1")
	}

	loc := time.Local
	date := time.Now().In(loc)
	if *dateStr != "" {
		t, err := time.ParseInLocation("2006-01-02", *dateStr, loc)
		if err != nil {
			fatal("invalid --date: %v", err)
		}
		date = t
	}
	start := date.AddDate(0, 0, -(*days - 1))

	email := os.Getenv("COROS_EMAIL")
	password := os.Getenv("COROS_PASSWORD")
	if email == "" || password == "" {
		fatal("COROS_EMAIL and COROS_PASSWORD env vars are required")
	}
	client, err := coros.New(coros.Config{
		Region:   *region,
		Email:    email,
		Password: password,
		UserID:   os.Getenv("COROS_USER_ID"),
	})
	if err != nil {
		fatal("client: %v", err)
	}

	// Ensure a single login happens before parallel fanout so goroutines
	// don't race on token/userId.
	if err := client.EnsureAuth(); err != nil {
		fatal("auth: %v", err)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    = map[string]string{}
		dayRes  *coros.DayDetailResult
		actRes  *coros.ActivitiesResult
		anaRes  *coros.Analytics
		schRes  *coros.Schedule
	)

	launch := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				errs[name] = err.Error()
				mu.Unlock()
			}
		}()
	}

	launch("dayDetail", func() error {
		r, err := client.DayDetail(date, start, date)
		if err == nil {
			dayRes = r
		}
		return err
	})
	launch("activities", func() error {
		r, err := client.QueryActivities(start, date, "")
		if err == nil {
			actRes = r
		}
		return err
	})
	launch("analytics", func() error {
		r, err := client.QueryAnalytics()
		if err == nil {
			anaRes = r
		}
		return err
	})
	launch("schedule", func() error {
		r, err := client.QueryTrainingSchedule(start, date)
		if err == nil {
			schRes = r
		}
		return err
	})
	wg.Wait()

	out := map[string]any{
		"date":         date.Format("2006-01-02"),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"region":       *region,
		"window_days":  *days,
	}
	if dayRes != nil {
		out["day"] = dayRes.Day
		out["range"] = dayRes.Range
		out["weekList"] = dayRes.WeekList
		out["derived"] = coros.ComputeDerived(dayRes.Range, dayRes.Day)
	}
	if actRes != nil {
		out["activities"] = actRes
	}
	if anaRes != nil {
		out["analytics"] = anaRes
	}
	if schRes != nil {
		out["schedule"] = schRes
	}
	if len(errs) > 0 {
		out["errors"] = errs
	}
	out["legend"] = legend

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fatal("encode: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
