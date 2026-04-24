package main

type tariffRates struct {
	ElectricityUnitRate       float64
	ElectricityStandingCharge float64
	ElectricityProductCode    string
	ElectricityTariffCode     string
	ElectricityIsAgile        bool
	GasUnitRate               float64
	GasStandingCharge         float64
}

func getRates(token string) (*tariffRates, error) {
	result, err := doGraphQL(gqlRequest{
		Query: `{ viewer { accounts { ... on AccountType { properties {
			electricityMeterPoints {
				agreements { validTo tariff {
					... on StandardTariff   { unitRate standingCharge productCode tariffCode }
					... on HalfHourlyTariff { standingCharge productCode tariffCode }
					... on PrepayTariff     { unitRate standingCharge productCode tariffCode }
				} }
			}
			gasMeterPoints {
				agreements { validTo tariff { unitRate standingCharge } }
			}
		} } } } }`,
	}, token)
	if err != nil {
		return nil, err
	}

	rates := &tariffRates{}

	accounts, _ := result["data"].(map[string]any)["viewer"].(map[string]any)["accounts"].([]any)
	for _, a := range accounts {
		props, _ := a.(map[string]any)["properties"].([]any)
		for _, p := range props {
			pm := p.(map[string]any)

			for _, mp := range toSlice(pm["electricityMeterPoints"]) {
				if tariff := activeAgreementTariff(mp); tariff != nil {
					rates.ElectricityUnitRate, _ = tariff["unitRate"].(float64)
					rates.ElectricityStandingCharge, _ = tariff["standingCharge"].(float64)
					rates.ElectricityProductCode, _ = tariff["productCode"].(string)
					rates.ElectricityTariffCode, _ = tariff["tariffCode"].(string)
					// HalfHourlyTariff has no unitRate field — detect Agile by absence.
					_, rates.ElectricityIsAgile = tariff["unitRates"]
					if _, hasUnit := tariff["unitRate"]; !hasUnit {
						rates.ElectricityIsAgile = true
					}
				}
			}

			for _, mp := range toSlice(pm["gasMeterPoints"]) {
				if tariff := activeAgreementTariff(mp); tariff != nil {
					rates.GasUnitRate, _ = tariff["unitRate"].(float64)
					rates.GasStandingCharge, _ = tariff["standingCharge"].(float64)
				}
			}
		}
	}

	return rates, nil
}

// activeAgreementTariff returns the tariff map for the agreement with validTo == null.
func activeAgreementTariff(meterPoint any) map[string]any {
	for _, ag := range toSlice(meterPoint.(map[string]any)["agreements"]) {
		agm := ag.(map[string]any)
		if agm["validTo"] == nil {
			tariff, _ := agm["tariff"].(map[string]any)
			return tariff
		}
	}
	return nil
}
