package coros

import (
	"net/http"
	"testing"
	"time"
)

func TestQueryActivities_Pagination(t *testing.T) {
	page := 0
	mock := newMock(t).on("/activity/query", func(w http.ResponseWriter, r *http.Request) {
		page++
		q := r.URL.Query()
		if q.Get("startDay") != "20260405" || q.Get("endDay") != "20260411" {
			t.Errorf("date params: start=%s end=%s", q.Get("startDay"), q.Get("endDay"))
		}
		if q.Get("size") != "200" {
			t.Errorf("size = %q", q.Get("size"))
		}
		switch page {
		case 1:
			if q.Get("pageNumber") != "1" {
				t.Errorf("page 1 pageNumber = %q", q.Get("pageNumber"))
			}
			envelope(w, "0000", map[string]any{
				"count":      2,
				"dataList":   []map[string]any{{"labelId": "a1", "date": 20260409, "name": "Alta Road Bike", "sportType": 200, "trainingLoad": 6}},
				"pageNumber": 1,
				"totalPage":  2,
			})
		case 2:
			if q.Get("pageNumber") != "2" {
				t.Errorf("page 2 pageNumber = %q", q.Get("pageNumber"))
			}
			envelope(w, "0000", map[string]any{
				"count":      1,
				"dataList":   []map[string]any{{"labelId": "a2", "date": 20260410, "name": "Recovery Ride", "sportType": 200, "trainingLoad": 4}},
				"pageNumber": 2,
				"totalPage":  2,
			})
		default:
			t.Errorf("unexpected page %d", page)
		}
	})
	c, _ := newTestClient(t, mock)

	start := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	res, err := c.QueryActivities(start, end, "")
	if err != nil {
		t.Fatalf("QueryActivities: %v", err)
	}
	if res.Count != 2 {
		t.Errorf("Count = %d, want 2", res.Count)
	}
	if len(res.Items) != 2 {
		t.Fatalf("Items len = %d, want 2", len(res.Items))
	}
	if res.Items[0].LabelID != "a1" || res.Items[1].LabelID != "a2" {
		t.Errorf("items = %+v", res.Items)
	}
	if page != 2 {
		t.Errorf("server hits = %d, want 2", page)
	}
}

func TestQueryActivities_Empty(t *testing.T) {
	mock := newMock(t).on("/activity/query", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, "0000", map[string]any{
			"count":      0,
			"dataList":   []any{},
			"pageNumber": 1,
			"totalPage":  0,
		})
	})
	c, _ := newTestClient(t, mock)
	start := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	res, err := c.QueryActivities(start, end, "")
	if err != nil {
		t.Fatalf("QueryActivities: %v", err)
	}
	if res.Count != 0 || len(res.Items) != 0 {
		t.Errorf("expected empty, got %+v", res)
	}
}
