FROM golang:1.22.5-alpine3.20

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
RUN go mod download

COPY ./test/e2e_globals.go ./e2e_globals.go
COPY ./test/init_test.go ./init_test.go
COPY ./test/utils.go ./utils.go
COPY ./test/fixtures ./fixtures

# Copy tests to run
COPY ./test/submit_single_test.go ./submit_01_single_test.go
COPY ./test/submit_batch_test.go ./submit_02_batch_test.go
COPY ./test/submit_double_spending_test.go ./submit_03_double_spending_test.go
COPY ./test/submit_beef_test.go ./submit_04_beef_test.go

# This will compile and run the tests
CMD [ "go", "test", "--tags=e2e", "-v", "-failfast", "./..."]
