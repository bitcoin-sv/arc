# Stage 1: Build the binaries
FROM golang:1.21.3-alpine AS build-stage

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

# Add grpc_health_probe if needed
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.24 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

# Build arc binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X $REPOSITORY/internal/version.Commit=$APP_COMMIT -X $REPOSITORY/internal/version.Version=$APP_VERSION" -o /arc_linux_amd64 ./cmd/arc/main.go

# Build broadcaster-cli binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /broadcaster-cli_linux_amd64 ./cmd/broadcaster-cli/main.go

# Stage 2: Prepare the final image
FROM alpine:latest AS prepare-stage

WORKDIR /service

# Create a minimal /etc/passwd file
RUN echo "nobody:x:65534:65534:nobody:/:" > /etc/passwd

COPY start.sh /service/start.sh
RUN chmod +x /service/start.sh
RUN ls -l /service/start.sh
RUN cat -v /service/start.sh  # Check for special characters in the script

# Stage 3: Create the final image
FROM scratch

WORKDIR /service

COPY --from=build-stage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-stage /arc_linux_amd64 /service/arc
COPY --from=build-stage /broadcaster-cli_linux_amd64 /service/broadcaster-cli
COPY --from=build-stage /bin/grpc_health_probe /bin/grpc_health_probe

# Copy the minimal /etc/passwd file
COPY --from=prepare-stage /etc/passwd /etc/passwd

# Copy the startup script from the prepare stage
COPY --from=prepare-stage /service/start.sh /service/start.sh

USER nobody

EXPOSE 9090

# Use the startup script as the entry point
CMD ["/service/start.sh"]
