package main

import (
	"fmt"
	"log"
	"os"
)

type meterKind string

const (
	electricity meterKind = "electricity"
	gas         meterKind = "gas"
)

type meterCandidate struct {
	kind     meterKind
	mpan     string
	mprn     string
	serial   string
	deviceID string
}

type resolvedMeter struct {
	deviceID string
	mpan     string // electricity
	mprn     string // gas
	serial   string
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
			gasMeterPoints {
				mprn
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

	data, _ := result["data"].(map[string]any)
	viewer, _ := data["viewer"].(map[string]any)
	accounts, _ := viewer["accounts"].([]any)
	for _, a := range accounts {
		props, _ := a.(map[string]any)["properties"].([]any)
		for _, p := range props {
			pm, ok := p.(map[string]any)
			if !ok {
				continue
			}

			for _, mp := range toSlice(pm["electricityMeterPoints"]) {
				mpPoint, ok := mp.(map[string]any)
				if !ok {
					continue
				}
				mpan, _ := mpPoint["mpan"].(string)
				for _, m := range toSlice(mpPoint["meters"]) {
					meterMap, ok := m.(map[string]any)
					if !ok {
						continue
					}
					serial, _ := meterMap["serialNumber"].(string)
					devices := toSlice(meterMap["smartDevices"])
					for _, d := range devices {
						dMap, ok := d.(map[string]any)
						if !ok {
							continue
						}
						deviceID, _ := dMap["deviceId"].(string)
						if deviceID != "" {
							candidates = append(candidates, meterCandidate{kind: electricity, mpan: mpan, serial: serial, deviceID: deviceID})
						}
					}
					// Include meters without smart devices so we can still use the REST consumption endpoint.
					if len(devices) == 0 && serial != "" {
						candidates = append(candidates, meterCandidate{kind: electricity, mpan: mpan, serial: serial})
					}
				}
			}

			for _, mp := range toSlice(pm["gasMeterPoints"]) {
				mpPoint, ok := mp.(map[string]any)
				if !ok {
					continue
				}
				mprn, _ := mpPoint["mprn"].(string)
				for _, m := range toSlice(mpPoint["meters"]) {
					meterMap, ok := m.(map[string]any)
					if !ok {
						continue
					}
					serial, _ := meterMap["serialNumber"].(string)
					devices := toSlice(meterMap["smartDevices"])
					for _, d := range devices {
						dMap, ok := d.(map[string]any)
						if !ok {
							continue
						}
						deviceID, _ := dMap["deviceId"].(string)
						if deviceID != "" {
							candidates = append(candidates, meterCandidate{kind: gas, mprn: mprn, serial: serial, deviceID: deviceID})
						}
					}
					if len(devices) == 0 && serial != "" {
						candidates = append(candidates, meterCandidate{kind: gas, mprn: mprn, serial: serial})
					}
				}
			}
		}
	}

	return candidates, nil
}

// resolveMeter finds the meter matching the env var filters for the given kind.
// Returns (nil, nil) if no meter of that kind exists on the account.
func resolveMeter(candidates []meterCandidate, kind meterKind) (*resolvedMeter, error) {
	var wantDeviceID, wantID, wantSerial string
	switch kind {
	case electricity:
		wantDeviceID = os.Getenv("OCTOPUS_DEVICE_ID")
		wantID = os.Getenv("OCTOPUS_MPAN")
		wantSerial = os.Getenv("OCTOPUS_SERIAL")
	case gas:
		wantDeviceID = os.Getenv("OCTOPUS_GAS_DEVICE_ID")
		wantID = os.Getenv("OCTOPUS_GAS_MPRN")
		wantSerial = os.Getenv("OCTOPUS_GAS_SERIAL")
	}

	for _, c := range candidates {
		if c.kind != kind {
			continue
		}
		if wantDeviceID != "" && c.deviceID != wantDeviceID {
			continue
		}
		if wantID != "" {
			if kind == electricity && c.mpan != wantID {
				continue
			}
			if kind == gas && c.mprn != wantID {
				continue
			}
		}
		if wantSerial != "" && c.serial != wantSerial {
			continue
		}

		m := &resolvedMeter{deviceID: c.deviceID, mpan: c.mpan, mprn: c.mprn, serial: c.serial}
		switch kind {
		case electricity:
			log.Printf("using electricity meter: MPAN=%s serial=%s deviceID=%s", m.mpan, m.serial, m.deviceID)
		case gas:
			log.Printf("using gas meter: MPRN=%s serial=%s deviceID=%s", m.mprn, m.serial, m.deviceID)
		}
		return m, nil
	}

	if wantDeviceID != "" || wantID != "" || wantSerial != "" {
		return nil, fmt.Errorf("no %s meter matched the specified filters", kind)
	}
	return nil, nil
}
