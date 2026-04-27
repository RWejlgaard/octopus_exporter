package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLatestConsumption_ReturnsLatestInterval(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":2,"results":[
			{"consumption":0.304,"interval_start":"2026-04-23T22:00:00+00:00","interval_end":"2026-04-23T22:30:00+00:00"},
			{"consumption":0.512,"interval_start":"2026-04-23T22:30:00+00:00","interval_end":"2026-04-23T23:00:00+00:00"}
		]}`)
	}))
	defer srv.Close()
	octopusREST = srv.URL

	c, err := getLatestConsumption(electricity, "MPAN123", "SERIAL456", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.KWh != 0.512 {
		t.Errorf("got kWh %v, want 0.512", c.KWh)
	}
}

func TestGetLatestConsumption_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":0,"results":[]}`)
	}))
	defer srv.Close()
	octopusREST = srv.URL

	_, err := getLatestConsumption(electricity, "MPAN123", "SERIAL456", "test")
	if err == nil {
		t.Error("expected error for empty results, got nil")
	}
}

func TestGetLatestConsumption_GasPath(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		fmt.Fprint(w, `{"count":1,"results":[
			{"consumption":1.23,"interval_start":"2026-04-23T22:00:00+00:00","interval_end":"2026-04-23T22:30:00+00:00"}
		]}`)
	}))
	defer srv.Close()
	octopusREST = srv.URL

	getLatestConsumption(gas, "MPRN789", "SERIAL456", "test")

	want := "/v1/gas-meter-points/MPRN789/meters/SERIAL456/consumption/"
	if capturedPath != want {
		t.Errorf("got path %q, want %q", capturedPath, want)
	}
}

func TestGetLatestConsumption_Non200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
	}))
	defer srv.Close()
	octopusREST = srv.URL

	_, err := getLatestConsumption(electricity, "MPAN123", "SERIAL456", "bad-key")
	if err == nil {
		t.Error("expected error for 401, got nil")
	}
}
