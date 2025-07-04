FROM debian:sid-slim AS build-stage

ARG TARGETARCH
ARG GOVERSION=1.24.4

# install tool-chain + Go
RUN apt-get update && apt-get install -y --no-install-recommends \
      wget build-essential ca-certificates g++ git pkg-config \
   && wget -qO- https://go.dev/dl/go${GOVERSION}.linux-${TARGETARCH}.tar.gz | tar -C /usr/local -xzf - \
   && rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-lstdc++"

ARG APP_COMMIT
ARG APP_VERSION
ARG REPOSITORY="github.com/bitcoin-sv/arc"
ARG MAIN="./cmd/arc/main.go"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
COPY config/ config/
COPY test/ test/

# Add grpc_health_probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.24 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

RUN go build \
     -ldflags "-X $REPOSITORY/internal/version.Commit=$APP_COMMIT -X $REPOSITORY/internal/version.Version=$APP_VERSION" \
     -o /arc_linux_amd64 $MAIN

# Build broadcaster-cli binary
RUN go build -o /broadcaster-cli_linux_amd64 ./cmd/broadcaster-cli/main.go

# Build e2e test binary
RUN go test --tags=e2e ./test -c -o /e2e_test.test

# Deploy the application binary into a lean image
FROM debian:sid-slim

WORKDIR /service

COPY --from=build-stage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-stage /arc_linux_amd64 /service/arc
COPY --from=build-stage /e2e_test.test /service/e2e_test.test
COPY --from=build-stage /broadcaster-cli_linux_amd64 /service/broadcaster-cli
COPY --from=build-stage /bin/grpc_health_probe /bin/grpc_health_probe
COPY deployments/passwd /etc/passwd

USER nobody

EXPOSE 9090

CMD ["/service/arc"]
