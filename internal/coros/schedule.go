package coros

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// WeekStageSum captures the plan-vs-actual totals for one week stage.
// Coros stores distances as strings in this response — we keep them as-is.
type WeekStageSum struct {
	PlanDistance       string `json:"planDistance"`
	PlanDuration       int    `json:"planDuration"`
	PlanElevGain       int    `json:"planElevGain"`
	PlanTrainingLoad   int    `json:"planTrainingLoad"`
	PlanSets           int    `json:"planSets"`
	ActualDistance     string `json:"actualDistance"`
	ActualDuration     int    `json:"actualDuration"`
	ActualElevGain     int    `json:"actualElevGain"`
	ActualTrainingLoad int    `json:"actualTrainingLoad"`
}

type WeekStage struct {
	FirstDayInWeek int          `json:"firstDayInWeek"`
	Stage          int          `json:"stage"`
	PlanID         string       `json:"planId,omitempty"`
	TrainSum       WeekStageSum `json:"trainSum"`
}

// Schedule is a subset of /training/schedule/query's data.
type Schedule struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	StartDay       int         `json:"startDay"`
	EndDay         int         `json:"endDay"`
	ExecuteStatus  int         `json:"executeStatus"`
	InSchedule     int         `json:"inSchedule"`
	Category       int         `json:"category"`
	Type           int         `json:"type"`
	UpdateTime     string      `json:"updateTime,omitempty"`
	WeekStages     []WeekStage `json:"weekStages,omitempty"`
	ProgramsCount  int         `json:"programsCount"` // derived
}

type scheduleRaw struct {
	Schedule
	Programs []any `json:"programs"`
}

// QueryTrainingSchedule calls /training/schedule/query for a date range.
// When the user has an active plan, Coros returns the plan object directly.
func (c *Client) QueryTrainingSchedule(start, end time.Time) (*Schedule, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}
	if c.userID == "" {
		return nil, fmt.Errorf("missing userId")
	}
	var raw scheduleRaw
	err := c.doWithRetry(func() (*http.Request, error) {
		u, _ := url.Parse(c.baseURL + "/training/schedule/query")
		q := u.Query()
		q.Set("startDate", formatDayInt(start))
		q.Set("endDate", formatDayInt(end))
		q.Set("supportRestExercise", "1")
		u.RawQuery = q.Encode()
		return http.NewRequest("GET", u.String(), nil)
	}, &raw)
	if err != nil {
		return nil, err
	}
	out := raw.Schedule
	out.ProgramsCount = len(raw.Programs)
	return &out, nil
}
