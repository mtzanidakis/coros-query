package coros

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// Zone is one band of a 1..5 zone classifier (e.g. trainingLoadRatioZoneList).
// Outermost zones may omit Min or Max, hence the pointers.
type Zone struct {
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	Type int      `json:"type"`
}

// Day mirrors one entry of /analyse/dayDetail/query's dayList.
// Field names match the Coros response (camelCase) so we can unmarshal and
// re-emit with the same tags.
type Day struct {
	HappenDay int    `json:"happenDay"`
	Timestamp int64  `json:"timestamp"`
	Date      string `json:"date,omitempty"` // derived: YYYY-MM-DD

	// Training load
	TrainingLoad              float64 `json:"trainingLoad"`
	TrainingLoadTarget        float64 `json:"trainingLoadTarget"`
	TrainingLoadRatio         float64 `json:"trainingLoadRatio"`
	TrainingLoadRatioState    int     `json:"trainingLoadRatioState"`
	TrainingLoadRatioZoneList []Zone  `json:"trainingLoadRatioZoneList,omitempty"`
	RecomendTlMin             float64 `json:"recomendTlMin"`
	RecomendTlMax             float64 `json:"recomendTlMax"`

	// Acute / chronic load curves
	T7D          int     `json:"t7d"`
	T28D         int     `json:"t28d"`
	Ct7dMin      float64 `json:"ct7dMin"`
	Ct7dMaxFixed float64 `json:"ct7dMaxFixed"`
	ATI          float64 `json:"ati"`
	CTI          float64 `json:"cti"`

	// Fatigue / freshness
	TIB                  float64 `json:"tib"`
	TiredRate            float64 `json:"tiredRate"`
	TiredRateNew         float64 `json:"tiredRateNew"`
	TiredRateStateNew    int     `json:"tiredRateStateNew"`
	TiredRateNewZoneList []Zone  `json:"tiredRateNewZoneList,omitempty"`

	// Heart
	RHR     int `json:"rhr"`
	TestRHR int `json:"testRhr"`

	// HRV (sleep-based)
	AvgSleepHrv          int   `json:"avgSleepHrv,omitempty"`
	SleepHrvBase         int   `json:"sleepHrvBase,omitempty"`
	SleepHrvIntervalList []int `json:"sleepHrvIntervalList,omitempty"`

	// Activity totals
	Distance       float64 `json:"distance"`
	DistanceTarget float64 `json:"distanceTarget"`
	Duration       int     `json:"duration"`
	DurationTarget int     `json:"durationTarget"`

	Performance int `json:"performance"`
}

// Week is one entry from dayDetail's weekList.
type Week struct {
	FirstDayOfWeek int     `json:"firstDayOfWeek"`
	RecomendTlMin  float64 `json:"recomendTlMin"`
	RecomendTlMax  float64 `json:"recomendTlMax"`
	TrainingLoad   int     `json:"trainingLoad"`
}

type dayDetailResponse struct {
	DayList  []Day  `json:"dayList"`
	WeekList []Week `json:"weekList"`
}

// DayDetailResult is what the CLI emits: the target day plus the full range
// the caller requested (e.g. a trailing window).
type DayDetailResult struct {
	Day      *Day   `json:"day"`
	Range    []Day  `json:"range"`
	WeekList []Week `json:"weekList,omitempty"`
}

// DayDetail calls /analyse/dayDetail/query?startDay=...&endDay=... and
// returns the window plus a pointer to the target day inside it.
func (c *Client) DayDetail(target, start, end time.Time) (*DayDetailResult, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}
	if c.userID == "" {
		return nil, fmt.Errorf("missing userId: set COROS_USER_ID env var (required for yfheader)")
	}

	var raw dayDetailResponse
	err := c.doWithRetry(func() (*http.Request, error) {
		u, _ := url.Parse(c.baseURL + "/analyse/dayDetail/query")
		q := u.Query()
		q.Set("startDay", formatDayInt(start))
		q.Set("endDay", formatDayInt(end))
		u.RawQuery = q.Encode()
		return http.NewRequest("GET", u.String(), nil)
	}, &raw)
	if err != nil {
		return nil, err
	}

	for i := range raw.DayList {
		raw.DayList[i].Date = dayIntToDate(raw.DayList[i].HappenDay)
	}
	sort.Slice(raw.DayList, func(i, j int) bool {
		return raw.DayList[i].HappenDay < raw.DayList[j].HappenDay
	})

	targetInt := dayInt(target)
	var targetDay *Day
	for i := range raw.DayList {
		if raw.DayList[i].HappenDay == targetInt {
			targetDay = &raw.DayList[i]
			break
		}
	}

	return &DayDetailResult{Day: targetDay, Range: raw.DayList, WeekList: raw.WeekList}, nil
}

func dayInt(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

func formatDayInt(t time.Time) string {
	return fmt.Sprintf("%08d", dayInt(t))
}

func dayIntToDate(n int) string {
	y := n / 10000
	m := (n / 100) % 100
	d := n % 100
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}
