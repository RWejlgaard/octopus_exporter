package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCurrentAgileRate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":1,"results":[
			{"value_exc_vat":21.68,"value_inc_vat":22.764,"valid_from":"2026-04-24T20:00:00Z","valid_to":"2026-04-24T20:30:00Z","payment_method":null}
		]}`)
	}))
	defer srv.Close()
	octopusREST = srv.URL

	rate, err := getCurrentAgileRate("AGILE-24-10-01", "E-1R-AGILE-24-10-01-C", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 22.764 {
		t.Errorf("got rate %v, want 22.764", rate)
	}
}

func TestGetCurrentAgileRate_NoSlot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":0,"results":[]}`)
	}))
	defer srv.Close()
	octopusREST = srv.URL

	_, err := getCurrentAgileRate("AGILE-24-10-01", "E-1R-AGILE-24-10-01-C", "test")
	if err == nil {
		t.Error("expected error for empty slot, got nil")
	}
}

func TestGetCurrentAgileRate_CorrectPath(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		fmt.Fprint(w, `{"count":1,"results":[{"value_inc_vat":22.764}]}`)
	}))
	defer srv.Close()
	octopusREST = srv.URL

	getCurrentAgileRate("AGILE-24-10-01", "E-1R-AGILE-24-10-01-C", "test")

	want := "/v1/products/AGILE-24-10-01/electricity-tariffs/E-1R-AGILE-24-10-01-C/standard-unit-rates/"
	if capturedPath != want {
		t.Errorf("got path %q, want %q", capturedPath, want)
	}
}
