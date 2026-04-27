package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestJsonFloat_Number(t *testing.T) {
	var f jsonFloat
	if err := json.Unmarshal([]byte(`42.5`), &f); err != nil {
		t.Fatal(err)
	}
	if float64(f) != 42.5 {
		t.Errorf("got %v, want 42.5", f)
	}
}

func TestJsonFloat_QuotedString(t *testing.T) {
	var f jsonFloat
	if err := json.Unmarshal([]byte(`"42.5"`), &f); err != nil {
		t.Fatal(err)
	}
	if float64(f) != 42.5 {
		t.Errorf("got %v, want 42.5", f)
	}
}

func TestJsonFloat_InvalidString(t *testing.T) {
	var f jsonFloat
	if err := json.Unmarshal([]byte(`"not-a-number"`), &f); err == nil {
		t.Error("expected error for non-numeric string, got nil")
	}
}

func TestToSlice_Slice(t *testing.T) {
	input := []any{1, 2, 3}
	got := toSlice(input)
	if len(got) != 3 {
		t.Errorf("got len %d, want 3", len(got))
	}
}

func TestToSlice_Nil(t *testing.T) {
	if got := toSlice(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestToSlice_WrongType(t *testing.T) {
	if got := toSlice("not a slice"); got != nil {
		t.Errorf("expected nil for wrong type, got %v", got)
	}
}

func TestExecuteWithRetry_RateLimitRetry(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `ok`)
	}))
	defer srv.Close()

	raw, err := executeWithRetry(func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, srv.URL, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != "ok" {
		t.Errorf("got body %q, want %q", string(raw), "ok")
	}
	if attempts.Load() != 3 {
		t.Errorf("got %d attempts, want 3", attempts.Load())
	}
}

func TestExecuteWithRetry_Non200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
	}))
	defer srv.Close()

	_, err := executeWithRetry(func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, srv.URL, nil)
	})
	if err == nil {
		t.Error("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to contain status code, got: %v", err)
	}
}

func TestExecuteWithRetry_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `internal server error`)
	}))
	defer srv.Close()

	_, err := executeWithRetry(func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, srv.URL, nil)
	})
	if err == nil {
		t.Error("expected error for 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status code, got: %v", err)
	}
}
