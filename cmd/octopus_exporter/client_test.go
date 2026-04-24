package main

import (
	"encoding/json"
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
