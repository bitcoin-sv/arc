FROM golang:1.24.1-bookworm AS builder

ARG APP_COMMIT
ARG APP_VERSION
ARG REPOSITORY="github.com/bitcoin-sv/arc"
ARG MAIN="./cmd/arc/main.go"

RUN apk --update add ca-certificates

RUN apk add --no-cache build-base gcompat

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
COPY config/ config/

# Add grpc_health_probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.24 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
     -ldflags "-X $REPOSITORY/internal/version.Commit=$APP_COMMIT -X $REPOSITORY/internal/version.Version=$APP_VERSION" \
     -o /arc_linux_amd64 $MAIN

# Build broadcaster-cli binary
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /broadcaster-cli_linux_amd64 ./cmd/broadcaster-cli/main.go

# Deploy the application binary into a lean image
FROM scratch

WORKDIR /service

COPY --from=build-stage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-stage /arc_linux_amd64 /service/arc
COPY --from=build-stage /broadcaster-cli_linux_amd64 /service/broadcaster-cli
COPY --from=build-stage /bin/grpc_health_probe /bin/grpc_health_probe
COPY deployments/passwd /etc/passwd

USER nobody

EXPOSE 9090

CMD ["/service/arc"]
