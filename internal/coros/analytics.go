package coros

import (
	"fmt"
	"net/http"
)

// SportStat is one row of /analyse/query's sportStatistic. SportType 65535 is
// the "all sports" aggregate; per-sport rows use the same codes as activities.
type SportStat struct {
	SportType    int     `json:"sportType"`
	Count        int     `json:"count"`
	Distance     float64 `json:"distance"`
	Duration     int     `json:"duration"`
	TrainingLoad int     `json:"trainingLoad"`
	AvgHeartRate int     `json:"avgHeartRate"`
	AvgSpeed     float64 `json:"avgSpeed,omitempty"`
}

// AreaEntry is one band of a zone-distribution histogram.
// Used by hrTimeAreaList, hrDisAreaList, hrTlAreaList, etc.
type AreaEntry struct {
	Index int     `json:"index"`
	Ratio float64 `json:"ratio"`
	Value float64 `json:"value"`
}

type SummaryInfo struct {
	HrTimeAreaList       []AreaEntry `json:"hrTimeAreaList,omitempty"`
	HrDisAreaList        []AreaEntry `json:"hrDisAreaList,omitempty"`
	HrTlAreaList         []AreaEntry `json:"hrTlAreaList,omitempty"`
	DistanceTimeAreaList []AreaEntry `json:"distanceTimeAreaList,omitempty"`
	DistanceCountAreaList []AreaEntry `json:"distanceCountAreaList,omitempty"`
	DistanceTlAreaList   []AreaEntry `json:"distanceTlAreaList,omitempty"`
	RecomendTlInDays     int         `json:"recomendTlInDays"`
}

// TlIntensityPeriod is one row of /analyse/query's tlIntensity.detailList,
// giving training load split by HR intensity band over a 4-week period.
type TlIntensityPeriod struct {
	FirstDayOfWeek  int     `json:"firstDayOfWeek"`
	LastDayInWeek   int     `json:"lastDayInWeek"`
	Value           int     `json:"value"`
	PeriodLowValue  int     `json:"periodLowValue"`
	PeriodLowPct    float64 `json:"periodLowPct"`
	PeriodMediumValue int   `json:"periodMediumValue"`
	PeriodMediumPct float64 `json:"periodMediumPct"`
	PeriodHighValue int     `json:"periodHighValue"`
	PeriodHighPct   float64 `json:"periodHighPct"`
}

type tlIntensityRaw struct {
	DetailList []TlIntensityPeriod `json:"detailList"`
}

type analyticsResponse struct {
	SportStatistic []SportStat    `json:"sportStatistic"`
	SummaryInfo    SummaryInfo    `json:"summaryInfo"`
	TlIntensity    tlIntensityRaw `json:"tlIntensity"`
}

// Analytics is the summarized output for the analytics section.
type Analytics struct {
	SportStatistic []SportStat         `json:"sportStatistic"`
	Summary        SummaryInfo         `json:"summary"`
	TlIntensity    []TlIntensityPeriod `json:"tlIntensity,omitempty"`
}

// QueryAnalytics calls /analyse/query (no params; ~12-week window) and
// extracts the aggregate/statistical sections.
func (c *Client) QueryAnalytics() (*Analytics, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}
	if c.userID == "" {
		return nil, fmt.Errorf("missing userId")
	}
	var raw analyticsResponse
	err := c.doWithRetry(func() (*http.Request, error) {
		return http.NewRequest("GET", c.baseURL+"/analyse/query", nil)
	}, &raw)
	if err != nil {
		return nil, err
	}
	return &Analytics{
		SportStatistic: raw.SportStatistic,
		Summary:        raw.SummaryInfo,
		TlIntensity:    raw.TlIntensity.DetailList,
	}, nil
}
