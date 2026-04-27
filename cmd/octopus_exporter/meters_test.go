package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveMeter_NoFilters_ReturnsFirst(t *testing.T) {
	t.Setenv("OCTOPUS_DEVICE_ID", "")
	t.Setenv("OCTOPUS_MPAN", "")
	t.Setenv("OCTOPUS_SERIAL", "")

	candidates := []meterCandidate{
		{kind: electricity, mpan: "1000000000001", serial: "A001", deviceID: "dev1"},
		{kind: electricity, mpan: "1000000000002", serial: "A002", deviceID: "dev2"},
	}
	m, err := resolveMeter(candidates, electricity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected meter, got nil")
	}
	if m.mpan != "1000000000001" {
		t.Errorf("got mpan %q, want 1000000000001", m.mpan)
	}
}

func TestResolveMeter_FilterByMPAN(t *testing.T) {
	t.Setenv("OCTOPUS_DEVICE_ID", "")
	t.Setenv("OCTOPUS_MPAN", "1000000000002")
	t.Setenv("OCTOPUS_SERIAL", "")

	candidates := []meterCandidate{
		{kind: electricity, mpan: "1000000000001", serial: "A001", deviceID: "dev1"},
		{kind: electricity, mpan: "1000000000002", serial: "A002", deviceID: "dev2"},
	}
	m, err := resolveMeter(candidates, electricity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected meter, got nil")
	}
	if m.mpan != "1000000000002" {
		t.Errorf("got mpan %q, want 1000000000002", m.mpan)
	}
}

func TestResolveMeter_FilterByDeviceID(t *testing.T) {
	t.Setenv("OCTOPUS_DEVICE_ID", "dev2")
	t.Setenv("OCTOPUS_MPAN", "")
	t.Setenv("OCTOPUS_SERIAL", "")

	candidates := []meterCandidate{
		{kind: electricity, mpan: "1000000000001", serial: "A001", deviceID: "dev1"},
		{kind: electricity, mpan: "1000000000002", serial: "A002", deviceID: "dev2"},
	}
	m, err := resolveMeter(candidates, electricity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.deviceID != "dev2" {
		t.Errorf("got deviceID %q, want dev2", m.deviceID)
	}
}

func TestResolveMeter_FilterMismatch_ReturnsError(t *testing.T) {
	t.Setenv("OCTOPUS_MPAN", "9999999999999")
	t.Setenv("OCTOPUS_DEVICE_ID", "")
	t.Setenv("OCTOPUS_SERIAL", "")

	candidates := []meterCandidate{
		{kind: electricity, mpan: "1000000000001", serial: "A001"},
	}
	_, err := resolveMeter(candidates, electricity)
	if err == nil {
		t.Error("expected error for unmatched filter, got nil")
	}
}

func TestResolveMeter_NoMetersOfKind_ReturnsNil(t *testing.T) {
	t.Setenv("OCTOPUS_DEVICE_ID", "")
	t.Setenv("OCTOPUS_MPAN", "")
	t.Setenv("OCTOPUS_SERIAL", "")

	candidates := []meterCandidate{
		{kind: gas, mprn: "1111111111", serial: "G001"},
	}
	m, err := resolveMeter(candidates, electricity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil for no electricity meters, got %+v", m)
	}
}

func TestResolveMeter_GasFilterByMPRN(t *testing.T) {
	t.Setenv("OCTOPUS_GAS_DEVICE_ID", "")
	t.Setenv("OCTOPUS_GAS_MPRN", "2222222222")
	t.Setenv("OCTOPUS_GAS_SERIAL", "")

	candidates := []meterCandidate{
		{kind: gas, mprn: "1111111111", serial: "G001"},
		{kind: gas, mprn: "2222222222", serial: "G002"},
	}
	m, err := resolveMeter(candidates, gas)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.mprn != "2222222222" {
		t.Errorf("got mprn %q, want 2222222222", m.mprn)
	}
}

func TestGetMeters_ElectricityWithSmartDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"properties":[{
			"electricityMeterPoints":[{
				"mpan":"1000000000001",
				"meters":[{"serialNumber":"A001","smartDevices":[{"deviceId":"dev1"}]}]
			}],
			"gasMeterPoints":[]
		}]}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	candidates, err := getMeters("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(candidates))
	}
	c := candidates[0]
	if c.kind != electricity {
		t.Errorf("kind: got %q, want electricity", c.kind)
	}
	if c.mpan != "1000000000001" {
		t.Errorf("mpan: got %q, want 1000000000001", c.mpan)
	}
	if c.deviceID != "dev1" {
		t.Errorf("deviceID: got %q, want dev1", c.deviceID)
	}
}

func TestGetMeters_MeterWithoutSmartDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"properties":[{
			"electricityMeterPoints":[{
				"mpan":"1000000000001",
				"meters":[{"serialNumber":"A001","smartDevices":[]}]
			}],
			"gasMeterPoints":[]
		}]}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	candidates, err := getMeters("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(candidates))
	}
	if candidates[0].deviceID != "" {
		t.Errorf("expected empty deviceID for meter without smart device, got %q", candidates[0].deviceID)
	}
}

func TestGetMeters_BothElectricityAndGas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"properties":[{
			"electricityMeterPoints":[{
				"mpan":"1000000000001",
				"meters":[{"serialNumber":"A001","smartDevices":[{"deviceId":"dev1"}]}]
			}],
			"gasMeterPoints":[{
				"mprn":"1111111111",
				"meters":[{"serialNumber":"G001","smartDevices":[{"deviceId":"gdev1"}]}]
			}]
		}]}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	candidates, err := getMeters("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(candidates))
	}
	kinds := map[meterKind]bool{}
	for _, c := range candidates {
		kinds[c.kind] = true
	}
	if !kinds[electricity] {
		t.Error("expected electricity candidate")
	}
	if !kinds[gas] {
		t.Error("expected gas candidate")
	}
}
