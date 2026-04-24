package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetKrakenToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"obtainKrakenToken":{"token":"test-token-123"}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	token, err := getKrakenToken("test-api-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("got token %q, want %q", token, "test-token-123")
	}
}

func TestGetKrakenToken_GraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"errors":[{"message":"Invalid API key"}]}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	_, err := getKrakenToken("bad-key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
