FROM golang:1.23-alpine3.21

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
COPY config/ config/
COPY test/ test/

# This will compile and run the tests
CMD [ "go", "test", "--tags=e2e", "-v", "-failfast", "./test/..."]
