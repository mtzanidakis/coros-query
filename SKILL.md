---
name: coros-query
description: Fetch a Coros Training Hub daily briefing (training load, fatigue, HRV, RHR, activities, active plan) as JSON for use in recovery/training recommendations.
---

# coros-query -- AI Agent Usage Guide

`coros-query` is a single-binary CLI that pulls a daily briefing from the private Coros Training Hub API and emits JSON to stdout. Use it when the user asks about their training readiness, recovery, fatigue, HRV, resting heart rate, recent activities, or weekly plan compliance.

## Prerequisites

The user must have set `COROS_EMAIL` and `COROS_PASSWORD` in the environment (typically via `.env` + direnv). `COROS_USER_ID` and `COROS_REGION` are optional. The tool caches its access token automatically, so repeated calls do not re-login.

You do NOT need to handle authentication, pagination, or retries — the CLI does all of that. Just run it and parse the JSON.

## Invocation

```
coros-query [--date YYYY-MM-DD] [--days N] [--region eu|us|cn]
```

| Flag | Default | Purpose |
|---|---|---|
| `--date` | today | Target day for the briefing |
| `--days` | `7` | Trailing window size ending at `--date`. Trend metrics (slope, monotony, RHR delta, days_since_hard) need at least 3–7 days to be meaningful |
| `--region` | `$COROS_REGION` or `eu` | Regional cluster |

Always pipe the JSON output into `jq` (or parse with code) before presenting anything to the user. Do not paste the raw JSON into your response — it is large and noisy.

## Output schema

Top-level keys:

| Key | Contents |
|---|---|
| `date` | Target date (`YYYY-MM-DD`) |
| `generated_at` | UTC timestamp of the run |
| `window_days` | Window size used |
| `day` | Object for the target date (see fields below). `null` if the day is outside the returned range |
| `range` | Array of day objects covering the trailing window, ascending |
| `weekList` | Per-week training load recommendations and actuals |
| `derived` | Client-side readiness metrics (see below) |
| `activities` | `{count, items[]}` — activities inside the window |
| `analytics` | Aggregated 12-week sport totals, HR zone distribution, training load intensity by period |
| `schedule` | Active training plan (may have empty `programs`) |
| `errors` | Present only on partial failure, keyed by section name |
| `legend` | Field glossary — read this first if you are unsure about a field name |

### Fields in `day` / `range`

Training load:
- `trainingLoad` — daily load (session impulse units)
- `trainingLoadRatio` — acute:chronic ratio (ACWR)
- `trainingLoadRatioState` — zone `1..5`: `1=low, 2=optimal-low, 3=optimal, 4=high, 5=very-high`
- `recomendTlMin` / `recomendTlMax` — recommended weekly training load floor/ceiling
- `t7d` / `t28d` — rolling 7-day / 28-day load sums
- `ati` / `cti` — acute / chronic training impact (~7d / ~28d EWMA)
- `tib` — training impact balance (`cti - ati`; `>0` fresh, `<0` fatigued)

Fatigue / freshness:
- `tiredRate` — legacy fatigue score (0..100)
- `tiredRateNew` — freshness/fatigue delta (`negative=fresh`, `positive=fatigued`)
- `tiredRateStateNew` — zone `1..5`: `1=very-fresh, 2=fresh, 3=balanced, 4=fatigued, 5=very-fatigued`

Heart:
- `rhr` — resting heart rate (bpm)
- `testRhr` — orthostatic test RHR (bpm)

HRV (overnight, RMSSD-style):
- `avgSleepHrv` — last night's HRV (ms)
- `sleepHrvBase` — personal baseline (ms)
- `sleepHrvIntervalList` — 4 thresholds `[low, balanced-low, balanced-high, high]`; the balanced band is `[intervals[2], intervals[3]]`

### Fields in `derived`

All optional — missing inputs yield `null`:

| Field | Meaning | Interpretation |
|---|---|---|
| `readiness_composite` | 0..100 composite | `>=80` ready, `60..80` moderate, `<60` caution |
| `hrv_zscore` | (today HRV - base) / balanced-band half-width | `>=0` at or above baseline, `<-1` concerning |
| `rhr_delta_7d` | today RHR − mean(prior days) | `<=-2` good recovery signal, `>=+5` stress/illness warning |
| `acute_chronic_ratio` | `t7d / (t28d/4)` | `0.8..1.3` sweet spot, `>1.5` overreach territory |
| `tl_trend_7d_slope` | linear slope of `t7d` | negative = tapering, positive = building |
| `training_monotony_7d` | Foster monotony `mean(TL)/stddev(TL)` | `>2` injury risk (too little variance) |
| `training_strain_7d` | Foster strain `sum(TL) * monotony` | compare across weeks, not absolute |
| `days_since_hard` | days since last `TrainingLoad > ct7dMin/7` | `0` = today, `>=4` = detraining drift |

## How to use the output

For a daily briefing, present (in order of importance):
1. `derived.readiness_composite` with a one-line interpretation
2. Key drivers: which inputs pushed the score up or down (HRV vs baseline, RHR delta, fatigue zone, TL ratio zone)
3. `day.trainingLoadRatioState` + `day.tiredRateStateNew` in plain language
4. Recent activities from `activities.items` (last 2–3 are usually enough)
5. Weekly plan compliance from `schedule.weekStages[0].trainSum` (planTrainingLoad vs actualTrainingLoad) if `schedule` is present
6. A training recommendation based on readiness, days_since_hard, ACWR, and the plan

Always reference the `legend` when writing explanations — it is the source of truth for field semantics and may be updated without changes to this guide.

## Sport type codes

Used in `activities.items[].sportType` and `analytics.sportStatistic[].sportType`:

| Code | Sport |
|---|---|
| 100 | Run |
| 101 | Indoor Run |
| 102 | Trail Run |
| 103 | Track Run |
| 200 | Road Bike |
| 201 | Indoor Bike |
| 203 | Gravel Bike |
| 204 | Mountain Bike |
| 300 | Pool Swim |
| 301 | Open Water |
| 402 | Strength |
| 900 | Walk |
| 65535 | All-sports aggregate (analytics only) |

## Units and encoding gotchas

- `distance` is meters; `duration` / `totalTime` / `workoutTime` are seconds
- `calorie` is `kcal × 1000` (divide by 1000 to get kcal)
- `activities.items[].avgSpeed` is `km/h × 100` when `speedType=1` (cycling); for running it is pace in a different scale — check `speedType` before converting
- `happenDay` and `date` in day objects are integers formatted `YYYYMMDD`; the helper field `day.date` provides `YYYY-MM-DD`
- `sleepHrvIntervalList` has exactly 4 values — index `[2]` and `[3]` are the balanced-band bounds (NOT `[1]` and `[2]`); the base `sleepHrvBase` falls inside `[intervals[2], intervals[3]]`

## Examples

```bash
# Default briefing for today
coros-query | jq '{date, readiness: .derived.readiness_composite, hrv: .day.avgSleepHrv, base: .day.sleepHrvBase, rhr: .day.rhr, tl_zone: .day.trainingLoadRatioState, fatigue_zone: .day.tiredRateStateNew}'

# Recovery check: only the fields an agent needs for a readiness verdict
coros-query | jq '{date, derived, today: {tl_state: .day.trainingLoadRatioState, fat_state: .day.tiredRateStateNew, hrv: .day.avgSleepHrv, hrv_base: .day.sleepHrvBase, rhr: .day.rhr}}'

# Specific past day
coros-query --date 2026-04-09

# Longer context window for weekly review
coros-query --days 14 | jq '.derived, .schedule.weekStages'

# Recent activities only
coros-query | jq '.activities.items[] | {date, name, sportType, distance, totalTime, avgHr, trainingLoad}'

# Plan compliance this week
coros-query | jq '.schedule.weekStages[0].trainSum'

# HR zone distribution from the 12-week analytics
coros-query | jq '.analytics.summary.hrTimeAreaList'
```

## Error handling

- Exit code `0` even on partial failures — check for an `errors` top-level key to see which sections failed.
- Exit code `1` with a message on stderr for hard failures (missing credentials, network down, login rejected). Report the stderr message verbatim to the user so they can fix their environment.
- If `day` is `null`, the requested date is outside the returned range (too old or future). Suggest a closer date.
- If `derived.*` fields are `null`, the window was too short for that metric — retry with a larger `--days`.

## What this CLI does NOT expose (yet)

If the user asks about any of these, explain that the endpoint is not currently wired up:

- Sleep duration and sleep-stage breakdown (deep/light/REM), sleep score
- Daytime stress / body battery
- VO2max trend (Running Fitness chart)
- Race predictor
- Threshold HR and pace zone configuration
- Personal records (running/cycling PRs)

For these, the user would need to capture the relevant endpoint from the Training Hub web UI (DevTools → Network) and request that it be added.
