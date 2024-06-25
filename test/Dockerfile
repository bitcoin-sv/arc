FROM golang:1.21.3

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
RUN go mod download

COPY ./init_test.go ./init_test.go
COPY ./utils.go ./utils.go
COPY ./fixtures ./fixtures

# Copy tests to run
COPY ./beef_test.go ./beef_test.go
COPY ./endpoint_test.go ./endpoint_test.go
COPY ./double_spend_test.go ./double_spend_test.go

# This will compile and run the tests
CMD [ "go", "test", "-v", "-failfast", "./..."]
