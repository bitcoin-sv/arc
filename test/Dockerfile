FROM golang:1.22.5-alpine3.20

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
# COPY test/ test/
# only these to speed up tests
COPY test/e2e_globals.go test/e2e_globals.go
COPY test/init_test.go test/init_test.go
COPY test/utils.go test/utils.go
COPY test/fixtures test/fixtures
COPY test/submit_05_reorg_test.go test/submit_05_reorg_test.go

# This will compile and run the tests
CMD [ "go", "test", "--tags=e2e", "-v", "-failfast", "./test/..."]
