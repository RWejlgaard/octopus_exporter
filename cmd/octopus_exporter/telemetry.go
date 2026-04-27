package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

type telemetryReading struct {
	ReadAt      string    `json:"readAt"`
	Consumption jsonFloat `json:"consumption"`
	Demand      jsonFloat `json:"demand"`
}

func getLiveConsumption(token, deviceID string) (*telemetryReading, error) {
	result, err := doGraphQL(gqlRequest{
		OperationName: "getSmartMeterTelemetry",
		Variables:     map[string]any{"meterDeviceId": deviceID},
		Query:         "query getSmartMeterTelemetry($meterDeviceId: String!, $start: DateTime, $end: DateTime, $grouping: TelemetryGrouping) {\n  smartMeterTelemetry(deviceId: $meterDeviceId, start: $start, end: $end, grouping: $grouping) {\n    readAt\n    consumption\n    demand\n    __typename\n  }\n}\n",
	}, token)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected API response: missing data field")
	}
	telemetry := toSlice(data["smartMeterTelemetry"])
	if len(telemetry) == 0 {
		return nil, errors.New("no telemetry data returned")
	}

	raw, err := json.Marshal(telemetry[0])
	if err != nil {
		return nil, err
	}
	var reading telemetryReading
	if err := json.Unmarshal(raw, &reading); err != nil {
		return nil, err
	}
	return &reading, nil
}
