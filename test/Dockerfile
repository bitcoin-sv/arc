FROM golang:1.21.3

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
RUN go mod download

COPY ./endpoint_test.go ./endpoint_test.go
COPY ./double_spend_test.go ./double_spend_test.go
COPY ./utils.go ./utils.go
COPY ./fixtures ./fixtures

# This will compile and run the tests
CMD [ "go", "test", "-v", "-failfast" ,"./..."]
