FROM golang:1.22.5-alpine3.20 AS build-stage

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
COPY config/ config/
COPY test/ test/

# Build tests
RUN go test --tags=e2e -v -failfast github.com/bitcoin-sv/arc/test -c -o /e2e.test

# Deploy the application binary into a lean image
FROM scratch

WORKDIR /test

COPY --from=build-stage /e2e.test /test/e2e.test

# Run tests
CMD ["/test/e2e.test"]
