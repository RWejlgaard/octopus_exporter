# octopus_exporter

A Prometheus exporter for live electricity consumption data from [Octopus Energy](https://octopus.energy), using the Kraken GraphQL API.

## Metrics

| Metric | Description |
|---|---|
| `octopus_live_consumption` | Live electricity demand in watts |
| `octopus_live_consumption_last_read` | Timestamp of the last meter reading (Unix epoch) |

Metrics are updated every 60 seconds.

## Configuration

| Variable | Required | Description |
|---|---|---|
| `OCTOPUS_API_KEY` | Yes | Your Octopus Energy API key |
| `OCTOPUS_MPAN` | No | Filter by MPAN (recommended if you have multiple meters) |
| `OCTOPUS_SERIAL` | No | Filter by meter serial number |
| `OCTOPUS_DEVICE_ID` | No | Use a specific smart device ID directly |
| `PORT` | No | Port to expose metrics on (default: `9359`) |

If none of `OCTOPUS_MPAN`, `OCTOPUS_SERIAL`, or `OCTOPUS_DEVICE_ID` are set, the exporter will automatically select the first smart meter found on your account. For accounts with multiple meters, use `OCTOPUS_MPAN` or `OCTOPUS_SERIAL` to pin to a specific one.

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
