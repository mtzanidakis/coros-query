package coros

import (
	"net/http"
	"testing"
	"time"
)

func TestQueryTrainingSchedule_ParsesPlan(t *testing.T) {
	mock := newMock(t).on("/training/schedule/query", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("startDate"); got != "20260411" {
			t.Errorf("startDate = %q", got)
		}
		if got := r.URL.Query().Get("endDate"); got != "20260411" {
			t.Errorf("endDate = %q", got)
		}
		if got := r.URL.Query().Get("supportRestExercise"); got != "1" {
			t.Errorf("supportRestExercise = %q", got)
		}
		envelope(w, "0000", map[string]any{
			"id":            "plan-1",
			"name":          "S4557",
			"startDay":      20241007,
			"endDay":        20241007,
			"executeStatus": 1,
			"programs":      []any{map[string]any{"x": 1}, map[string]any{"x": 2}},
			"weekStages": []map[string]any{
				{
					"firstDayInWeek": 20260406,
					"stage":          0,
					"planId":         "plan-1",
					"trainSum": map[string]any{
						"planDistance":       "0",
						"planDuration":       0,
						"planTrainingLoad":   0,
						"actualDistance":     "0",
						"actualDuration":     0,
						"actualTrainingLoad": 311,
					},
				},
			},
		})
	})
	c, _ := newTestClient(t, mock)

	d := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	res, err := c.QueryTrainingSchedule(d, d)
	if err != nil {
		t.Fatalf("QueryTrainingSchedule: %v", err)
	}
	if res.Name != "S4557" || res.ID != "plan-1" {
		t.Errorf("plan = %+v", res)
	}
	if res.ProgramsCount != 2 {
		t.Errorf("ProgramsCount = %d, want 2", res.ProgramsCount)
	}
	if len(res.WeekStages) != 1 || res.WeekStages[0].TrainSum.ActualTrainingLoad != 311 {
		t.Errorf("WeekStages = %+v", res.WeekStages)
	}
}
