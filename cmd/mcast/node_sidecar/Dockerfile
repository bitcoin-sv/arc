FROM golang:1.23.5-alpine3.20 AS build-stage

ARG APP_COMMIT
ARG APP_VERSION
ARG REPOSITORY="github.com/bitcoin-sv/arc"

RUN apk --update add ca-certificates

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

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X $REPOSITORY/internal/version.Commit=$APP_COMMIT -X $REPOSITORY/internal/version.Version=$APP_VERSION" -o /bsvn_sidecar_linux_amd64 ./cmd/mcast/node_sidecar/main.go


# Deploy the application binary into a lean image
FROM scratch

WORKDIR /service

COPY --from=build-stage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-stage /bsvn_sidecar_linux_amd64 /service/sidecar
COPY --from=build-stage /bin/grpc_health_probe /bin/grpc_health_probe

EXPOSE 9090

CMD ["/service/sidecar"]

# Todo: remove this file and combine with ~/Dockerfile with arguments
