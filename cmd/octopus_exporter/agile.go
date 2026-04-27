package main

import (
	"errors"
	"fmt"
	"net/url"
	"time"
)

// getCurrentAgileRate returns the unit rate (inc. VAT, pence/kWh) for the current
// half-hour slot from the Agile tariff REST endpoint.
func getCurrentAgileRate(productCode, tariffCode, key string) (float64, error) {
	now := time.Now().UTC()
	slotStart := now.Truncate(30 * time.Minute)
	slotEnd := slotStart.Add(30 * time.Minute)

	path := fmt.Sprintf("/v1/products/%s/electricity-tariffs/%s/standard-unit-rates/", productCode, tariffCode)
	result, err := doREST(path, url.Values{
		"period_from": {slotStart.Format(time.RFC3339)},
		"period_to":   {slotEnd.Format(time.RFC3339)},
	}, key)
	if err != nil {
		return 0, err
	}

	results := toSlice(result["results"])
	if len(results) == 0 {
		return 0, errors.New("no Agile rate found for current slot")
	}

	rate, ok := results[0].(map[string]any)["value_inc_vat"].(float64)
	if !ok {
		return 0, errors.New("value_inc_vat missing from Agile rate response")
	}
	return rate, nil
}
