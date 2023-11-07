FROM golang:1.21.3

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
RUN go mod download

COPY ./arc_txt_endpoint_test.go ./arc_txt_endpoint_test.go
COPY ./utils.go ./utils.go

# This will compile and run the tests
CMD [ "go", "test", "-v" ,"./..."]
