package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	apiKey string
	port   string
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func gauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
}

func main() {
	apiKey = mustEnv("OCTOPUS_API_KEY")
	port = envOrDefault("PORT", "9359")

	token, err := getKrakenToken(apiKey)
	if err != nil {
		log.Fatalf("failed to get initial token: %v", err)
	}

	elecMeter, err := resolveMeter(token, electricity)
	if err != nil {
		log.Fatalf("failed to resolve electricity meter: %v", err)
	}
	if elecMeter == nil {
		log.Fatal("no electricity smart meter found on account")
	}

	gasMeter, err := resolveMeter(token, gas)
	if err != nil {
		log.Fatalf("failed to resolve gas meter: %v", err)
	}
	if gasMeter == nil {
		log.Println("no gas smart meter found — gas metrics disabled")
	}

	// --- Metrics ---

	// Electricity telemetry (live, from GraphQL)
	elecDemand := gauge("octopus_electricity_demand_watts", "Live electricity demand in watts")
	elecLastRead := gauge("octopus_electricity_last_read_timestamp", "Unix timestamp of last electricity reading")

	// Electricity consumption (half-hourly kWh, from REST)
	elecConsumption := gauge("octopus_electricity_consumption_kwh", "Half-hourly electricity consumption in kWh")
	elecConsumptionInterval := gauge("octopus_electricity_consumption_interval_timestamp", "Unix timestamp of the start of the latest consumption interval")

	// Electricity tariff
	elecUnitRate := gauge("octopus_electricity_unit_rate_pence", "Current electricity unit rate in pence per kWh")
	elecStandingCharge := gauge("octopus_electricity_standing_charge_pence", "Current electricity standing charge in pence per day")

	// Account
	accountBalance := gauge("octopus_account_balance_pence", "Account balance in pence (positive = credit, negative = debit)")

	toRegister := []prometheus.Collector{
		elecDemand, elecLastRead,
		elecConsumption, elecConsumptionInterval,
		elecUnitRate, elecStandingCharge,
		accountBalance,
	}

	var (
		gasDemand            prometheus.Gauge
		gasLastRead          prometheus.Gauge
		gasConsumption       prometheus.Gauge
		gasConsumptionInterval prometheus.Gauge
		gasUnitRate          prometheus.Gauge
		gasStandCharge       prometheus.Gauge
	)
	if gasMeter != nil {
		gasDemand = gauge("octopus_gas_demand_watts", "Live gas demand in watts")
		gasLastRead = gauge("octopus_gas_last_read_timestamp", "Unix timestamp of last gas reading")
		gasConsumption = gauge("octopus_gas_consumption_kwh", "Half-hourly gas consumption in kWh")
		gasConsumptionInterval = gauge("octopus_gas_consumption_interval_timestamp", "Unix timestamp of the start of the latest gas consumption interval")
		gasUnitRate = gauge("octopus_gas_unit_rate_pence", "Current gas unit rate in pence per kWh")
		gasStandCharge = gauge("octopus_gas_standing_charge_pence", "Current gas standing charge in pence per day")
		toRegister = append(toRegister, gasDemand, gasLastRead, gasConsumption, gasConsumptionInterval, gasUnitRate, gasStandCharge)
	}

	prometheus.MustRegister(toRegister...)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("serving metrics on :%s/metrics", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}()

	for {
		// Electricity telemetry (live demand)
		if elecMeter.deviceID != "" {
			reading, err := getLiveConsumption(token, elecMeter.deviceID)
			if err != nil {
				log.Printf("electricity telemetry error: %v", err)
				if token, err = getKrakenToken(apiKey); err != nil {
					log.Printf("token refresh failed: %v", err)
				}
			} else {
				elecDemand.Set(float64(reading.Demand))
				if t, err := time.Parse("2006-01-02T15:04:05+00:00", reading.ReadAt); err == nil {
					elecLastRead.Set(float64(t.Unix()))
				}
			}
		}

		// Gas telemetry (live demand)
		if gasMeter != nil && gasMeter.deviceID != "" {
			reading, err := getLiveConsumption(token, gasMeter.deviceID)
			if err != nil {
				log.Printf("gas telemetry error: %v", err)
			} else {
				gasDemand.Set(float64(reading.Demand))
				if t, err := time.Parse("2006-01-02T15:04:05+00:00", reading.ReadAt); err == nil {
					gasLastRead.Set(float64(t.Unix()))
				}
			}
		}

		// Electricity half-hourly consumption (REST)
		if elecMeter.mpan != "" && elecMeter.serial != "" {
			c, err := getLatestConsumption(electricity, elecMeter.mpan, elecMeter.serial)
			if err != nil {
				log.Printf("electricity consumption error: %v", err)
			} else {
				elecConsumption.Set(c.KWh)
				elecConsumptionInterval.Set(float64(c.IntervalStart.Unix()))
			}
		}

		// Gas half-hourly consumption (REST)
		if gasMeter != nil && gasMeter.mprn != "" && gasMeter.serial != "" {
			c, err := getLatestConsumption(gas, gasMeter.mprn, gasMeter.serial)
			if err != nil {
				log.Printf("gas consumption error: %v", err)
			} else {
				gasConsumption.Set(c.KWh)
				gasConsumptionInterval.Set(float64(c.IntervalStart.Unix()))
			}
		}

		// Tariff rates
		rates, err := getRates(token)
		if err != nil {
			log.Printf("rates error: %v", err)
		} else {
			unitRate := rates.ElectricityUnitRate
			if rates.ElectricityIsAgile && rates.ElectricityProductCode != "" && rates.ElectricityTariffCode != "" {
				agileRate, err := getCurrentAgileRate(rates.ElectricityProductCode, rates.ElectricityTariffCode)
				if err != nil {
					log.Printf("agile rate error: %v", err)
				} else {
					unitRate = agileRate
				}
			}
			elecUnitRate.Set(unitRate)
			elecStandingCharge.Set(rates.ElectricityStandingCharge)

			if gasMeter != nil {
				gasUnitRate.Set(rates.GasUnitRate)
				gasStandCharge.Set(rates.GasStandingCharge)
			}
		}

		// Account balance
		balance, err := getAccountBalance(token)
		if err != nil {
			log.Printf("account balance error: %v", err)
		} else {
			accountBalance.Set(balance)
		}

		time.Sleep(60 * time.Second)
	}
}
