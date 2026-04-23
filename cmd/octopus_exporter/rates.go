package main

type tariffRates struct {
	ElectricityUnitRate       float64
	ElectricityStandingCharge float64
	GasUnitRate               float64
	GasStandingCharge         float64
}

// electricityTariffFragments covers all known electricity tariff union types.
const electricityTariffFragments = `
	... on StandardTariff   { unitRate standingCharge }
	... on HalfHourlyTariff { standingCharge }
	... on PrepayTariff     { unitRate standingCharge }
`

func getRates(token string) (*tariffRates, error) {
	result, err := doGraphQL(gqlRequest{
		Query: `{ viewer { accounts { ... on AccountType { properties {
			electricityMeterPoints {
				agreements { validTo tariff {` + electricityTariffFragments + `} }
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
