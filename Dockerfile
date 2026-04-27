FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /octopus_exporter ./cmd/octopus_exporter

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /octopus_exporter /octopus_exporter
EXPOSE 9359
ENTRYPOINT ["/octopus_exporter"]
