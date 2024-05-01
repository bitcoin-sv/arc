FROM golang:1.21.3-alpine AS build-stage

RUN apk --update add ca-certificates

WORKDIR /app

COPY . .

RUN go mod download
RUN go mod verify
# Add grpc_health_probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.24 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /arc_linux_amd64 ./cmd/arc/main.go
# RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -v -o /arc_linux_amd64 ./cmd/arc/main.go

# Deploy the application binary into a lean image
FROM scratch

WORKDIR /service

COPY --from=build-stage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-stage /arc_linux_amd64 /service/arc
COPY --from=build-stage /bin/grpc_health_probe /bin/grpc_health_probe

# RUN chmod +x arc

EXPOSE 9090

CMD ["/service/arc"]
