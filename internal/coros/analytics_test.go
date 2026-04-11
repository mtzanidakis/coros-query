package coros

import (
	"net/http"
	"testing"
)

func TestQueryAnalytics_ExtractsSections(t *testing.T) {
	mock := newMock(t).on("/analyse/query", func(w http.ResponseWriter, r *http.Request) {
		envelope(w, "0000", map[string]any{
			"sportStatistic": []map[string]any{
				{"sportType": 200, "count": 38, "distance": 593848.12, "duration": 68615, "trainingLoad": 1728, "avgHeartRate": 134, "avgSpeed": 31.16},
				{"sportType": 65535, "count": 38, "distance": 593848.12, "duration": 68615, "trainingLoad": 1728, "avgHeartRate": 134},
			},
			"summaryInfo": map[string]any{
				"hrTimeAreaList": []map[string]any{
					{"index": 0, "ratio": 44.3, "value": 46687.0},
					{"index": 1, "ratio": 21.0, "value": 22138.0},
				},
				"recomendTlInDays": 0,
			},
			"tlIntensity": map[string]any{
				"detailList": []map[string]any{
					{"firstDayOfWeek": 20260406, "lastDayInWeek": 20260412, "value": 311, "periodLowValue": 111, "periodLowPct": 35.69, "periodMediumValue": 197, "periodMediumPct": 63.34, "periodHighValue": 3, "periodHighPct": 0.97},
				},
			},
		})
	})
	c, _ := newTestClient(t, mock)

	res, err := c.QueryAnalytics()
	if err != nil {
		t.Fatalf("QueryAnalytics: %v", err)
	}
	if len(res.SportStatistic) != 2 {
		t.Errorf("SportStatistic len = %d", len(res.SportStatistic))
	}
	if res.SportStatistic[0].SportType != 200 || res.SportStatistic[0].TrainingLoad != 1728 {
		t.Errorf("SportStatistic[0] = %+v", res.SportStatistic[0])
	}
	if len(res.Summary.HrTimeAreaList) != 2 {
		t.Errorf("HrTimeAreaList len = %d", len(res.Summary.HrTimeAreaList))
	}
	if res.Summary.HrTimeAreaList[0].Ratio != 44.3 {
		t.Errorf("HrTimeAreaList[0].Ratio = %v", res.Summary.HrTimeAreaList[0].Ratio)
	}
	if len(res.TlIntensity) != 1 || res.TlIntensity[0].Value != 311 {
		t.Errorf("TlIntensity = %+v", res.TlIntensity)
	}
}
