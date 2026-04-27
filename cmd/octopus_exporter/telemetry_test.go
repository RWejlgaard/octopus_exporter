package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLiveConsumption_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"smartMeterTelemetry":[
			{"readAt":"2026-04-24T10:00:00Z","consumption":"1.23","demand":"456.7"}
		]}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	reading, err := getLiveConsumption("test-token", "device-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if float64(reading.Demand) != 456.7 {
		t.Errorf("demand: got %v, want 456.7", reading.Demand)
	}
	if float64(reading.Consumption) != 1.23 {
		t.Errorf("consumption: got %v, want 1.23", reading.Consumption)
	}
	if reading.ReadAt != "2026-04-24T10:00:00Z" {
		t.Errorf("readAt: got %q, want %q", reading.ReadAt, "2026-04-24T10:00:00Z")
	}
}

func TestGetLiveConsumption_EmptyTelemetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"smartMeterTelemetry":[]}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	_, err := getLiveConsumption("test-token", "device-123")
	if err == nil {
		t.Error("expected error for empty telemetry, got nil")
	}
}

func TestGetLiveConsumption_MissingDataField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	_, err := getLiveConsumption("test-token", "device-123")
	if err == nil {
		t.Error("expected error for null data, got nil")
	}
}

func TestGetLiveConsumption_GraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"errors":[{"message":"device not found"}]}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	_, err := getLiveConsumption("test-token", "device-123")
	if err == nil {
		t.Error("expected error for GraphQL error, got nil")
	}
}
