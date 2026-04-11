package coros

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Activity mirrors one item of /activity/query's dataList. We only keep the
// fields that are useful for a daily briefing; the rest is ignored.
//
// Unit notes (from live inspection):
//   - distance: meters (float)
//   - totalTime / workoutTime: seconds
//   - calorie: kcal × 1000
//   - avgSpeed (speedType=1, cycling): km/h × 100. For running it's pace; check speedType.
//   - ascent / descent: meters
type Activity struct {
	LabelID     string `json:"labelId"`
	Name        string `json:"name,omitempty"`
	Date        int    `json:"date"` // YYYYMMDD
	StartTime   int64  `json:"startTime"`
	EndTime     int64  `json:"endTime"`
	SportType   int    `json:"sportType"`
	Mode        int    `json:"mode"`
	SubMode     int    `json:"subMode"`
	SpeedType   int    `json:"speedType"`

	Distance float64 `json:"distance"`
	Ascent   int     `json:"ascent"`
	Descent  int     `json:"descent"`

	TotalTime   int `json:"totalTime"`
	WorkoutTime int `json:"workoutTime"`

	AvgHr    int     `json:"avgHr"`
	AvgPower int     `json:"avgPower"`
	NP       int     `json:"np"`
	AvgSpeed float64 `json:"avgSpeed"`
	MaxSpeed int     `json:"maxSpeed"`
	AvgCadence int   `json:"avgCadence"`

	TrainingLoad int `json:"trainingLoad"`
	Calorie      int `json:"calorie"` // kcal × 1000
}

// ActivitiesResult is what the CLI emits for the activities section.
type ActivitiesResult struct {
	Count int        `json:"count"`
	Items []Activity `json:"items"`
}

type activitiesResponse struct {
	Count      int        `json:"count"`
	DataList   []Activity `json:"dataList"`
	PageNumber int        `json:"pageNumber"`
	TotalPage  int        `json:"totalPage"`
}

// QueryActivities fetches all activities in [start, end] (inclusive), paginating
// until exhaustion. modeList may be empty to include all sport types.
func (c *Client) QueryActivities(start, end time.Time, modeList string) (*ActivitiesResult, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}
	if c.userID == "" {
		return nil, fmt.Errorf("missing userId")
	}

	var all []Activity
	page := 1
	for {
		var raw activitiesResponse
		err := c.doWithRetry(func() (*http.Request, error) {
			u, _ := url.Parse(c.baseURL + "/activity/query")
			q := u.Query()
			q.Set("size", "200")
			q.Set("pageNumber", strconv.Itoa(page))
			q.Set("startDay", formatDayInt(start))
			q.Set("endDay", formatDayInt(end))
			q.Set("modeList", modeList)
			u.RawQuery = q.Encode()
			return http.NewRequest("GET", u.String(), nil)
		}, &raw)
		if err != nil {
			return nil, err
		}
		all = append(all, raw.DataList...)
		if raw.TotalPage <= page || len(raw.DataList) == 0 {
			break
		}
		page++
	}
	return &ActivitiesResult{Count: len(all), Items: all}, nil
}
