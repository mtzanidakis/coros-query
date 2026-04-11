# coros-query

CLI that fetches a daily training briefing from the Coros Training Hub and emits it as JSON, intended for consumption by an LLM agent or scripted analysis.

Coros does not publish an API. This tool talks to the same private endpoints the [Training Hub web app](https://t.coros.com) uses. Credentials are yours; the tool just drives the web app from the command line.

## Installation

```bash
go install github.com/mtzanidakis/coros-query@latest
```

Or download a binary from the [releases page](https://github.com/mtzanidakis/coros-query/releases).

## Configuration

The tool reads credentials from environment variables. Copy `.env.example` to `.env` and fill it in, then `direnv allow` (a `.envrc` is included).

| Variable | Required | Description |
|---|---|---|
| `COROS_EMAIL` | yes | Training Hub account email |
| `COROS_PASSWORD` | yes | Training Hub password |
| `COROS_USER_ID` | no | Numeric user ID used in the `yfheader` header. Auto-detected from the login response when possible; set it explicitly if auto-detection fails |
| `COROS_REGION` | no | `eu`, `us`, or `cn` (default: `eu`) |

The access token is cached at `$XDG_CACHE_HOME/coros-query/token-<region>.json` and reused across runs until it fails, at which point the tool re-logs in automatically.

## Usage

```
coros-query [--date YYYY-MM-DD] [--days N] [--region eu|us|cn]
```

| Flag | Default | Description |
|---|---|---|
| `--date` | today | Target date |
| `--days` | `7` | Trailing window size ending at `--date`. Needed for trend metrics (slope, monotony, RHR delta) |
| `--region` | `$COROS_REGION` or `eu` | Coros regional cluster |

### Examples

```bash
# Today's briefing with 7-day context
coros-query

# A specific date
coros-query --date 2026-04-11

# Longer context window for trend analysis
coros-query --days 14

# Just the target day (strip everything else)
coros-query --date 2026-04-11 | jq .day

# Essentials only for an LLM prompt
coros-query | jq '{date, day, derived, schedule}'
```

## Output

All output is JSON on stdout; errors go to stderr. Sections:

| Section | Source endpoint | Contents |
|---|---|---|
| `day` | `/analyse/dayDetail/query` | Target day's metrics: training load, fatigue, HRV, RHR, zones |
| `range` | `/analyse/dayDetail/query` | Trailing `--days` window (same shape as `day`, one entry per day) |
| `weekList` | `/analyse/dayDetail/query` | Per-week training load recommendations |
| `derived` | computed client-side | Readiness composite and trends (see below) |
| `activities` | `/activity/query` | Activities inside the window (paginated) |
| `analytics` | `/analyse/query` | 12-week sport totals + HR zone distribution + training load intensity by period |
| `schedule` | `/training/schedule/query` | Active training plan + weekly plan-vs-actual totals |
| `errors` | — | Per-section failures; present only if something went wrong |
| `legend` | — | Field glossary |

Sections are fetched concurrently, and failures are reported per section — one broken endpoint does not block the others.

### Derived metrics

Computed from the `range` window, with `nil` for metrics that need more data than is available.

| Metric | Meaning |
|---|---|
| `readiness_composite` | 0..100 weighted mix of `tiredRateNew` zone (35%), HRV z-score (25%), RHR delta (15%), `trainingLoadRatio` zone (25%) |
| `hrv_zscore` | `(todayHRV - baseline) / balanced-band half-width`. Negative means below baseline |
| `rhr_delta_7d` | Today's RHR minus the mean of the prior days in the window |
| `tl_trend_7d_slope` | Linear regression slope of `t7d` over the window. Negative means acute load is tapering |
| `acute_chronic_ratio` | `t7d / (t28d/4)`. `0.8..1.3` is the sweet spot, `>1.5` is overreach territory |
| `training_monotony_7d` | Foster monotony: `mean(TL) / stddev(TL)`. `>2` indicates lack of variance (injury risk) |
| `training_strain_7d` | Foster strain: `sum(TL) * monotony` |
| `days_since_hard` | Days since the last session with `trainingLoad > ct7dMin/7` |

## Building from source

Requires Go 1.26+.

```bash
make build              # -> bin/coros-query
make test               # go test ./...
make vet                # go vet ./...
make fmt                # gofmt -w .
make run ARGS="--date 2026-04-11"
make clean
```

## Releases

Tagged versions (`v*`) trigger a [GoReleaser](https://goreleaser.com/) build on GitHub Actions, producing archives for Linux/macOS/Windows × amd64/arm64. Push a tag to cut a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Limitations

- The `/analyse/query` endpoint returns a fixed ~12-week trailing window; it is not paginated.
- `training/schedule/query` returns the user's active plan. Per-day scheduled workouts are exposed only when the plan has `programs` populated.
- Sleep duration/quality, VO2max trend, race predictor, HR/pace threshold configuration, and personal records live behind separate endpoints that are not yet wired up.

## License

MIT
