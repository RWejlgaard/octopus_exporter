# octopus_exporter

A Prometheus exporter for Octopus Energy smart meter data, using the Kraken GraphQL API.

## Metrics

### Electricity

| Metric | Source | Description |
|---|---|---|
| `octopus_electricity_demand_watts` | GraphQL | Live electricity demand in watts |
| `octopus_electricity_last_read_timestamp` | GraphQL | Unix timestamp of last electricity reading |
| `octopus_electricity_consumption_kwh` | REST | Latest half-hourly consumption in kWh |
| `octopus_electricity_consumption_interval_timestamp` | REST | Unix timestamp of the start of the latest consumption interval |
| `octopus_electricity_unit_rate_pence` | GraphQL / REST | Current unit rate in pence per kWh (Agile customers get the live half-hourly rate from the REST API) |
| `octopus_electricity_standing_charge_pence` | GraphQL | Current standing charge in pence per day |

### Gas

Gas metrics are only exposed if a smart gas meter is found on the account.

| Metric | Source | Description |
|---|---|---|
| `octopus_gas_demand_watts` | GraphQL | Live gas demand in watts |
| `octopus_gas_last_read_timestamp` | GraphQL | Unix timestamp of last gas reading |
| `octopus_gas_consumption_kwh` | REST | Latest half-hourly consumption in kWh |
| `octopus_gas_consumption_interval_timestamp` | REST | Unix timestamp of the start of the latest gas consumption interval |
| `octopus_gas_unit_rate_pence` | GraphQL | Current unit rate in pence per kWh |
| `octopus_gas_standing_charge_pence` | GraphQL | Current standing charge in pence per day |

### Account

| Metric | Source | Description |
|---|---|---|
| `octopus_account_balance_pence` | GraphQL | Account balance in pence (positive = credit, negative = debit) |

Metrics are updated every 60 seconds.

## Configuration

| Variable | Required | Description |
|---|---|---|
| `OCTOPUS_API_KEY` | Yes | Your Octopus Energy API key |
| `OCTOPUS_MPAN` | No | Filter electricity meter by MPAN |
| `OCTOPUS_SERIAL` | No | Filter electricity meter by serial number |
| `OCTOPUS_DEVICE_ID` | No | Use a specific electricity smart device ID directly |
| `OCTOPUS_GAS_MPRN` | No | Filter gas meter by MPRN |
| `OCTOPUS_GAS_SERIAL` | No | Filter gas meter by serial number |
| `OCTOPUS_GAS_DEVICE_ID` | No | Use a specific gas smart device ID directly |
| `PORT` | No | Port to expose metrics on (default: `9359`) |

If no filter variables are set, the exporter auto-discovers the first smart meter of each type found on the account. Use `OCTOPUS_MPAN` / `OCTOPUS_MPRN` to pin to a specific meter on accounts with multiple meters.

Your API key can be found in the [Octopus Energy developer dashboard](https://octopus.energy/dashboard/new/accounts/personal-details/api-access).

## Docker

```sh
docker run -d \
  -e OCTOPUS_API_KEY=sk_live_... \
  -e OCTOPUS_MPAN=1234567890123 \
  -p 9359:9359 \
  rwejlgaard/octopus_exporter
```

## Running from source

Requires Go 1.22+.

```sh
git clone https://github.com/rwejlgaard/octopus_exporter
cd octopus_exporter
OCTOPUS_API_KEY=sk_live_... go run ./cmd/octopus_exporter
```

## Prometheus configuration

```yaml
scrape_configs:
  - job_name: octopus
    static_configs:
      - targets: ['localhost:9359']
```

## Building

```sh
go build ./cmd/octopus_exporter
```

```sh
docker build -t rwejlgaard/octopus_exporter .
```
