package main

import (
	"errors"
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

	accounts, _ := result["data"].(map[string]any)["viewer"].(map[string]any)["accounts"].([]any)
	for _, a := range accounts {
		props, _ := a.(map[string]any)["properties"].([]any)
		for _, p := range props {
			pm := p.(map[string]any)

			for _, mp := range toSlice(pm["electricityMeterPoints"]) {
				mpan, _ := mp.(map[string]any)["mpan"].(string)
				for _, m := range toSlice(mp.(map[string]any)["meters"]) {
					serial, _ := m.(map[string]any)["serialNumber"].(string)
					for _, d := range toSlice(m.(map[string]any)["smartDevices"]) {
						deviceID, _ := d.(map[string]any)["deviceId"].(string)
						if deviceID != "" {
							candidates = append(candidates, meterCandidate{kind: electricity, mpan: mpan, serial: serial, deviceID: deviceID})
						}
					}
				}
			}

			for _, mp := range toSlice(pm["gasMeterPoints"]) {
				mprn, _ := mp.(map[string]any)["mprn"].(string)
				for _, m := range toSlice(mp.(map[string]any)["meters"]) {
					serial, _ := m.(map[string]any)["serialNumber"].(string)
					for _, d := range toSlice(m.(map[string]any)["smartDevices"]) {
						deviceID, _ := d.(map[string]any)["deviceId"].(string)
						if deviceID != "" {
							candidates = append(candidates, meterCandidate{kind: gas, mprn: mprn, serial: serial, deviceID: deviceID})
						}
					}
				}
			}
		}
	}

	return candidates, nil
}

// resolveDeviceID finds the device ID for the given meter kind using environment
// variable filters. Returns ("", nil) if no meter of that kind exists on the account.
func resolveDeviceID(token string, kind meterKind) (string, error) {
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

	if wantDeviceID != "" && wantID == "" && wantSerial == "" {
		return wantDeviceID, nil
	}

	log.Printf("discovering %s meters from account...", kind)
	candidates, err := getMeters(token)
	if err != nil {
		return "", err
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
		switch kind {
		case electricity:
			log.Printf("using electricity meter: MPAN=%s serial=%s deviceID=%s", c.mpan, c.serial, c.deviceID)
		case gas:
			log.Printf("using gas meter: MPRN=%s serial=%s deviceID=%s", c.mprn, c.serial, c.deviceID)
		}
		return c.deviceID, nil
	}

	if wantDeviceID != "" || wantID != "" || wantSerial != "" {
		return "", fmt.Errorf("no %s meter matched the specified filters", kind)
	}

	// No filters set and no meter found — this kind may not be on the account.
	filtered := 0
	for _, c := range candidates {
		if c.kind == kind {
			filtered++
		}
	}
	if filtered == 0 {
		return "", nil
	}
	return "", errors.New("unexpected: meters found but none selected")
}
