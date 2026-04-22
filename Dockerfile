FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -o /octopus_exporter ./cmd/octopus_exporter

FROM scratch
COPY --from=builder /octopus_exporter /octopus_exporter
EXPOSE 9359
ENTRYPOINT ["/octopus_exporter"]
