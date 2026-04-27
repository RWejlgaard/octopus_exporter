package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
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

func counter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{Name: name, Help: help})
}

func main() {
	apiKey = mustEnv("OCTOPUS_API_KEY")
	port = envOrDefault("PORT", "9359")

	token, err := getKrakenToken(apiKey)
	if err != nil {
		log.Fatalf("failed to get initial token: %v", err)
	}

	log.Println("discovering meters from account...")
	candidates, err := getMeters(token)
	if err != nil {
		log.Fatalf("failed to discover meters: %v", err)
	}

	elecMeter, err := resolveMeter(candidates, electricity)
	if err != nil {
		log.Fatalf("failed to resolve electricity meter: %v", err)
	}
	if elecMeter == nil {
		log.Fatal("no electricity smart meter found on account")
	}

	gasMeter, err := resolveMeter(candidates, gas)
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

	// Exporter health
	exporterUp := gauge("octopus_up", "1 if the last poll cycle completed without errors, 0 otherwise")
	pollErrors := counter("octopus_poll_errors_total", "Total number of collector errors per poll cycle")
	tokenRefreshCount := counter("octopus_token_refreshes_total", "Total number of successful JWT token refreshes")
	rateLimitRetries = counter("octopus_rate_limit_retries_total", "Total number of 429 rate-limit retries across all requests")

	toRegister := []prometheus.Collector{
		elecDemand, elecLastRead,
		elecConsumption, elecConsumptionInterval,
		elecUnitRate, elecStandingCharge,
		accountBalance,
		exporterUp, pollErrors, tokenRefreshCount, rateLimitRetries,
	}

	var (
		gasDemand              prometheus.Gauge
		gasLastRead            prometheus.Gauge
		gasConsumption         prometheus.Gauge
		gasConsumptionInterval prometheus.Gauge
		gasUnitRate            prometheus.Gauge
		gasStandCharge         prometheus.Gauge
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

	// tokenMu guards token across concurrent poll goroutines.
	var tokenMu sync.RWMutex

	// withToken calls fn with the current token, refreshing once on JWT expiry.
	withToken := func(fn func(string) error) error {
		tokenMu.RLock()
		t := token
		tokenMu.RUnlock()

		err := fn(t)
		if !errors.Is(err, errTokenExpired) {
			return err
		}

		// Only one goroutine refreshes; others will pick up the new token.
		tokenMu.Lock()
		if token == t {
			newT, e := getKrakenToken(apiKey)
			if e != nil {
				tokenMu.Unlock()
				log.Printf("token refresh failed: %v", e)
				return err
			}
			token = newT
			tokenRefreshCount.Inc()
		}
		newT := token
		tokenMu.Unlock()

		return fn(newT)
	}

	for {
		var (
			wg        sync.WaitGroup
			failedAny atomic.Bool
		)

		fail := func(format string, args ...any) {
			log.Printf(format, args...)
			pollErrors.Inc()
			failedAny.Store(true)
		}

		collect := func(name string, fn func() error) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fn(); err != nil {
					fail("%s error: %v", name, err)
				}
			}()
		}

		// Electricity telemetry (live demand)
		if elecMeter.deviceID != "" {
			collect("electricity telemetry", func() error {
				return withToken(func(t string) error {
					reading, err := getLiveConsumption(t, elecMeter.deviceID)
					if err != nil {
						return err
					}
					elecDemand.Set(float64(reading.Demand))
					if ts, err := time.Parse(time.RFC3339, reading.ReadAt); err == nil {
						elecLastRead.Set(float64(ts.Unix()))
					}
					return nil
				})
			})
		}

		// Gas telemetry (live demand)
		if gasMeter != nil && gasMeter.deviceID != "" {
			collect("gas telemetry", func() error {
				return withToken(func(t string) error {
					reading, err := getLiveConsumption(t, gasMeter.deviceID)
					if err != nil {
						return err
					}
					gasDemand.Set(float64(reading.Demand))
					if ts, err := time.Parse(time.RFC3339, reading.ReadAt); err == nil {
						gasLastRead.Set(float64(ts.Unix()))
					}
					return nil
				})
			})
		}

		// Electricity half-hourly consumption (REST)
		if elecMeter.mpan != "" && elecMeter.serial != "" {
			collect("electricity consumption", func() error {
				c, err := getLatestConsumption(electricity, elecMeter.mpan, elecMeter.serial, apiKey)
				if err != nil {
					return err
				}
				elecConsumption.Set(c.KWh)
				elecConsumptionInterval.Set(float64(c.IntervalStart.Unix()))
				return nil
			})
		}

		// Gas half-hourly consumption (REST)
		if gasMeter != nil && gasMeter.mprn != "" && gasMeter.serial != "" {
			collect("gas consumption", func() error {
				c, err := getLatestConsumption(gas, gasMeter.mprn, gasMeter.serial, apiKey)
				if err != nil {
					return err
				}
				gasConsumption.Set(c.KWh)
				gasConsumptionInterval.Set(float64(c.IntervalStart.Unix()))
				return nil
			})
		}

		// Tariff rates (result needed for optional agile lookup after wg.Wait)
		var collectedRates *tariffRates
		collect("rates", func() error {
			return withToken(func(t string) error {
				r, err := getRates(t)
				if err != nil {
					return err
				}
				collectedRates = r
				return nil
			})
		})

		// Account balance
		collect("account balance", func() error {
			return withToken(func(t string) error {
				balance, err := getAccountBalance(t)
				if err != nil {
					return err
				}
				accountBalance.Set(balance)
				return nil
			})
		})

		wg.Wait()

		// Agile rate depends on the rates result, so it runs after the parallel phase.
		if collectedRates != nil {
			unitRate := collectedRates.ElectricityUnitRate
			if collectedRates.ElectricityIsAgile && collectedRates.ElectricityProductCode != "" && collectedRates.ElectricityTariffCode != "" {
				agileRate, err := getCurrentAgileRate(collectedRates.ElectricityProductCode, collectedRates.ElectricityTariffCode, apiKey)
				if err != nil {
					fail("agile rate error: %v", err)
				} else {
					unitRate = agileRate
				}
			}
			elecUnitRate.Set(unitRate)
			elecStandingCharge.Set(collectedRates.ElectricityStandingCharge)
			if gasMeter != nil {
				gasUnitRate.Set(collectedRates.GasUnitRate)
				gasStandCharge.Set(collectedRates.GasStandingCharge)
			}
		}

		if failedAny.Load() {
			exporterUp.Set(0)
		} else {
			exporterUp.Set(1)
		}

		time.Sleep(60 * time.Second)
	}
}
