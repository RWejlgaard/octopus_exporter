package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAccountBalance_Credit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"balance":4250}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	balance, err := getAccountBalance("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != 4250 {
		t.Errorf("got %v, want 4250", balance)
	}
}

func TestGetAccountBalance_Debit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"balance":-1500}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	balance, err := getAccountBalance("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balance != -1500 {
		t.Errorf("got %v, want -1500", balance)
	}
}

func TestGetAccountBalance_Missing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	_, err := getAccountBalance("token")
	if err == nil {
		t.Error("expected error when balance is missing, got nil")
	}
}
