package coros

import (
	"net/http"
	"testing"
	"time"
)

func TestDayDetail_ParsesAndFiltersTarget(t *testing.T) {
	mock := newMock(t).on("/analyse/dayDetail/query", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("startDay"); got != "20260405" {
			t.Errorf("startDay = %q", got)
		}
		if got := r.URL.Query().Get("endDay"); got != "20260411" {
			t.Errorf("endDay = %q", got)
		}
		envelope(w, "0000", map[string]any{
			"dayList": []map[string]any{
				{"happenDay": 20260411, "trainingLoad": 0.0, "rhr": 51, "avgSleepHrv": 25, "sleepHrvBase": 26, "sleepHrvIntervalList": []int{5, 19, 22, 30}},
				{"happenDay": 20260410, "trainingLoad": 0.0, "rhr": 58},
				{"happenDay": 20260409, "trainingLoad": 6.0, "rhr": 51},
			},
			"weekList": []map[string]any{
				{"firstDayOfWeek": 20260406, "recomendTlMin": 462.0, "recomendTlMax": 693.0, "trainingLoad": 311},
			},
		})
	})
	c, _ := newTestClient(t, mock)

	target := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	res, err := c.DayDetail(target, start, target)
	if err != nil {
		t.Fatalf("DayDetail: %v", err)
	}
	if res.Day == nil {
		t.Fatal("Day is nil")
	}
	if res.Day.HappenDay != 20260411 {
		t.Errorf("Day.HappenDay = %d", res.Day.HappenDay)
	}
	if res.Day.Date != "2026-04-11" {
		t.Errorf("Day.Date = %q", res.Day.Date)
	}
	if len(res.Range) != 3 {
		t.Errorf("Range len = %d, want 3", len(res.Range))
	}
	// Range must be ascending by happenDay
	for i := 1; i < len(res.Range); i++ {
		if res.Range[i-1].HappenDay > res.Range[i].HappenDay {
			t.Errorf("Range not sorted ascending at %d", i)
		}
	}
	if len(res.WeekList) != 1 || res.WeekList[0].TrainingLoad != 311 {
		t.Errorf("WeekList = %+v", res.WeekList)
	}
}

func TestDayDetail_TargetMissingYieldsNilDay(t *testing.T) {
	mock := newMock(t).on("/analyse/dayDetail/query", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, "0000", map[string]any{
			"dayList": []map[string]any{
				{"happenDay": 20260410, "rhr": 55},
			},
		})
	})
	c, _ := newTestClient(t, mock)
	target := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	res, err := c.DayDetail(target, target, target)
	if err != nil {
		t.Fatalf("DayDetail: %v", err)
	}
	if res.Day != nil {
		t.Errorf("Day = %+v, want nil", res.Day)
	}
	if len(res.Range) != 1 {
		t.Errorf("Range len = %d, want 1", len(res.Range))
	}
}

func TestDayDetail_MissingUserIDErrors(t *testing.T) {
	c, _ := newTestClient(t, newMock(t))
	c.userID = ""
	target := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	_, err := c.DayDetail(target, target, target)
	if err == nil {
		t.Fatal("want error when userID missing")
	}
}

func TestDayIntHelpers(t *testing.T) {
	tt := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	if got := dayInt(tt); got != 20260411 {
		t.Errorf("dayInt = %d", got)
	}
	if got := formatDayInt(tt); got != "20260411" {
		t.Errorf("formatDayInt = %q", got)
	}
	if got := dayIntToDate(20260411); got != "2026-04-11" {
		t.Errorf("dayIntToDate = %q", got)
	}
	if got := dayIntToDate(20260101); got != "2026-01-01" {
		t.Errorf("zero-pad dayIntToDate = %q", got)
	}
}
