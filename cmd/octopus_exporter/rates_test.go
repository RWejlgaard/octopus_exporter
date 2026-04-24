package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActiveAgreementTariff_ActiveFound(t *testing.T) {
	mp := map[string]any{
		"agreements": []any{
			map[string]any{"validTo": "2020-01-01T00:00:00Z", "tariff": map[string]any{"unitRate": 10.0}},
			map[string]any{"validTo": nil, "tariff": map[string]any{"unitRate": 26.3}},
		},
	}
	tariff := activeAgreementTariff(mp)
	if tariff == nil {
		t.Fatal("expected tariff, got nil")
	}
	if tariff["unitRate"].(float64) != 26.3 {
		t.Errorf("got unitRate %v, want 26.3", tariff["unitRate"])
	}
}

func TestActiveAgreementTariff_NoneActive(t *testing.T) {
	mp := map[string]any{
		"agreements": []any{
			map[string]any{"validTo": "2020-01-01T00:00:00Z", "tariff": map[string]any{"unitRate": 10.0}},
		},
	}
	if tariff := activeAgreementTariff(mp); tariff != nil {
		t.Errorf("expected nil, got %v", tariff)
	}
}

func TestActiveAgreementTariff_NoAgreements(t *testing.T) {
	mp := map[string]any{"agreements": []any{}}
	if tariff := activeAgreementTariff(mp); tariff != nil {
		t.Errorf("expected nil, got %v", tariff)
	}
}

func TestGetRates_StandardTariff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"properties":[{
			"electricityMeterPoints":[{"agreements":[{"validTo":null,"tariff":{
				"unitRate":26.32,"standingCharge":54.55,"productCode":"VAR-22-11-01","tariffCode":"E-1R-VAR-22-11-01-C"
			}}]}],
			"gasMeterPoints":[]
		}]}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	rates, err := getRates("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rates.ElectricityUnitRate != 26.32 {
		t.Errorf("unit rate: got %v, want 26.32", rates.ElectricityUnitRate)
	}
	if rates.ElectricityStandingCharge != 54.55 {
		t.Errorf("standing charge: got %v, want 54.55", rates.ElectricityStandingCharge)
	}
	if rates.ElectricityTariffCode != "E-1R-VAR-22-11-01-C" {
		t.Errorf("tariff code: got %q, want %q", rates.ElectricityTariffCode, "E-1R-VAR-22-11-01-C")
	}
	if rates.ElectricityIsAgile {
		t.Error("expected IsAgile=false for StandardTariff")
	}
}

func TestGetRates_AgileTariff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"properties":[{
			"electricityMeterPoints":[{"agreements":[{"validTo":null,"tariff":{
				"standingCharge":54.55,"productCode":"AGILE-24-10-01","tariffCode":"E-1R-AGILE-24-10-01-C"
			}}]}],
			"gasMeterPoints":[]
		}]}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	rates, err := getRates("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rates.ElectricityIsAgile {
		t.Error("expected IsAgile=true for HalfHourlyTariff (no unitRate field)")
	}
	if rates.ElectricityProductCode != "AGILE-24-10-01" {
		t.Errorf("product code: got %q, want %q", rates.ElectricityProductCode, "AGILE-24-10-01")
	}
}

func TestGetRates_WithGas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"viewer":{"accounts":[{"properties":[{
			"electricityMeterPoints":[{"agreements":[{"validTo":null,"tariff":{
				"unitRate":26.32,"standingCharge":54.55,"productCode":"VAR-22-11-01","tariffCode":"E-1R-VAR-22-11-01-C"
			}}]}],
			"gasMeterPoints":[{"agreements":[{"validTo":null,"tariff":{
				"unitRate":7.22,"standingCharge":29.11
			}}]}]
		}]}]}}}`)
	}))
	defer srv.Close()
	octopusGraphQL = srv.URL + "/"

	rates, err := getRates("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rates.GasUnitRate != 7.22 {
		t.Errorf("gas unit rate: got %v, want 7.22", rates.GasUnitRate)
	}
	if rates.GasStandingCharge != 29.11 {
		t.Errorf("gas standing charge: got %v, want 29.11", rates.GasStandingCharge)
	}
}
