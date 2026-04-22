package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const octopusGraphQL = "https://api.octopus.energy/v1/graphql/"

var (
	apiKey = mustEnv("OCTOPUS_API_KEY")
	port   = envOrDefault("PORT", "9359")
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type gqlRequest struct {
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables"`
	Query         string         `json:"query"`
}

func doGraphQL(req gqlRequest, authToken string) (map[string]any, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, octopusGraphQL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		httpReq.Header.Set("Authorization", "JWT "+authToken)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	if errs, ok := result["errors"].([]any); ok && len(errs) > 0 {
		if e, ok := errs[0].(map[string]any); ok {
			return nil, fmt.Errorf("GraphQL error: %s", e["message"])
		}
		return nil, errors.New("GraphQL error")
	}

	return result, nil
}

func getKrakenToken(apiKey string) (string, error) {
	result, err := doGraphQL(gqlRequest{
		Variables: map[string]any{"apikey": apiKey},
		Query: `mutation krakenTokenAuthentication($apikey: String!) {
			obtainKrakenToken(input: {APIKey: $apikey}) {
				token
			}
		}`,
	}, "")
	if err != nil {
		return "", err
	}

	token, ok := result["data"].(map[string]any)["obtainKrakenToken"].(map[string]any)["token"].(string)
	if !ok {
		return "", errors.New("token not found in response")
	}
	return token, nil
}

type meterCandidate struct {
	mpan     string
	serial   string
	deviceID string
}

func getMeters(token string) ([]meterCandidate, error) {
	result, err := doGraphQL(gqlRequest{
		Query: `{ viewer { accounts { ... on AccountType { properties {
			electricityMeterPoints {
				mpan
				meters {
					serialNumber
					smartDevices { deviceId }
				}
			}
		} } } } }`,
	}, token)
	if err != nil {
		return nil, err
	}

	var candidates []meterCandidate

	accounts, _ := result["data"].(map[string]any)["viewer"].(map[string]any)["accounts"].([]any)
	for _, a := range accounts {
		props, _ := a.(map[string]any)["properties"].([]any)
		for _, p := range props {
			mps, _ := p.(map[string]any)["electricityMeterPoints"].([]any)
			for _, mp := range mps {
				mpan, _ := mp.(map[string]any)["mpan"].(string)
				meters, _ := mp.(map[string]any)["meters"].([]any)
				for _, m := range meters {
					serial, _ := m.(map[string]any)["serialNumber"].(string)
					devices, _ := m.(map[string]any)["smartDevices"].([]any)
					for _, d := range devices {
						deviceID, _ := d.(map[string]any)["deviceId"].(string)
						if deviceID != "" {
							candidates = append(candidates, meterCandidate{
								mpan:     mpan,
								serial:   serial,
								deviceID: deviceID,
							})
						}
					}
				}
			}
		}
	}

	return candidates, nil
}

func resolveDeviceID(token string) (string, error) {
	wantDeviceID := os.Getenv("OCTOPUS_DEVICE_ID")
	wantMPAN := os.Getenv("OCTOPUS_MPAN")
	wantSerial := os.Getenv("OCTOPUS_SERIAL")

	if wantDeviceID != "" && wantMPAN == "" && wantSerial == "" {
		return wantDeviceID, nil
	}

	log.Println("discovering meters from account...")
	candidates, err := getMeters(token)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", errors.New("no smart meters found on account")
	}

	for _, c := range candidates {
		if wantDeviceID != "" && c.deviceID != wantDeviceID {
			continue
		}
		if wantMPAN != "" && c.mpan != wantMPAN {
			continue
		}
		if wantSerial != "" && c.serial != wantSerial {
			continue
		}
		log.Printf("using meter: MPAN=%s serial=%s deviceID=%s", c.mpan, c.serial, c.deviceID)
		return c.deviceID, nil
	}

	return "", fmt.Errorf("no meter matched OCTOPUS_DEVICE_ID=%q OCTOPUS_MPAN=%q OCTOPUS_SERIAL=%q", wantDeviceID, wantMPAN, wantSerial)
}

type telemetryReading struct {
	ReadAt      string  `json:"readAt"`
	Consumption float64 `json:"consumption"`
	Demand      float64 `json:"demand"`
}

var errTokenExpired = errors.New("token expired")

func getLiveConsumption(token, deviceID string) (*telemetryReading, error) {
	result, err := doGraphQL(gqlRequest{
		OperationName: "getSmartMeterTelemetry",
		Variables:     map[string]any{"meterDeviceId": deviceID},
		Query:         "query getSmartMeterTelemetry($meterDeviceId: String!, $start: DateTime, $end: DateTime, $grouping: TelemetryGrouping) {\n  smartMeterTelemetry(deviceId: $meterDeviceId, start: $start, end: $end, grouping: $grouping) {\n    readAt\n    consumption\n    demand\n    __typename\n  }\n}\n",
	}, token)
	if err != nil {
		if err.Error() == "GraphQL error: Signature of the JWT has expired." {
			return nil, errTokenExpired
		}
		return nil, err
	}

	telemetry, ok := result["data"].(map[string]any)["smartMeterTelemetry"].([]any)
	if !ok || len(telemetry) == 0 {
		return nil, errors.New("no data found")
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

func main() {
	liveConsumption := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "octopus_live_consumption",
		Help: "Octopus Energy live consumption in watts",
	})
	lastRead := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "octopus_live_consumption_last_read",
		Help: "Octopus Energy live consumption last read in seconds since epoch",
	})
	prometheus.MustRegister(liveConsumption, lastRead)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("serving metrics on :%s/metrics", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}()

	token, err := getKrakenToken(apiKey)
	if err != nil {
		log.Fatalf("failed to get initial token: %v", err)
	}

	deviceID, err := resolveDeviceID(token)
	if err != nil {
		log.Fatalf("failed to resolve device ID: %v", err)
	}

	for {
		reading, err := getLiveConsumption(token, deviceID)
		if err != nil {
			log.Printf("error fetching telemetry: %v", err)
			token, err = getKrakenToken(apiKey)
			if err != nil {
				log.Printf("failed to refresh token: %v", err)
			}
		} else {
			liveConsumption.Set(reading.Demand)

			t, err := time.Parse("2006-01-02T15:04:05+00:00", reading.ReadAt)
			if err != nil {
				log.Printf("failed to parse readAt %q: %v", reading.ReadAt, err)
			} else {
				lastRead.Set(float64(t.Unix()))
			}
		}
		time.Sleep(60 * time.Second)
	}
}
