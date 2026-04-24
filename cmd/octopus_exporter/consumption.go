package main

import (
	"errors"
	"fmt"
	"net/url"
	"time"
)

type consumptionReading struct {
	KWh           float64
	IntervalStart time.Time
}

func getLatestConsumption(kind meterKind, id, serial string) (*consumptionReading, error) {
	var path string
	switch kind {
	case electricity:
		path = fmt.Sprintf("/v1/electricity-meter-points/%s/meters/%s/consumption/", id, serial)
	case gas:
		path = fmt.Sprintf("/v1/gas-meter-points/%s/meters/%s/consumption/", id, serial)
	}

	// Consumption data can lag several hours, so use a 24h window and take the latest entry.
	result, err := doREST(path, url.Values{
		"period_from": {time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)},
		"order_by":    {"period"},
	})
	if err != nil {
		return nil, err
	}

	results := toSlice(result["results"])
	if len(results) == 0 {
		return nil, errors.New("no consumption data in last 24h")
	}

	latest := results[len(results)-1].(map[string]any)
	kwh, _ := latest["consumption"].(float64)
	start, err := time.Parse(time.RFC3339, latest["interval_start"].(string))
	if err != nil {
		return nil, fmt.Errorf("failed to parse interval_start: %w", err)
	}

	return &consumptionReading{KWh: kwh, IntervalStart: start}, nil
}
