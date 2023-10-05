# Start from a Debian base image
FROM golang:1.21.1

# Set the Current Working Directory inside the container
WORKDIR /app


# Download and install Bitcoin CLI for amd64
RUN wget https://bitcoincore.org/bin/bitcoin-core-0.21.1/bitcoin-0.21.1-x86_64-linux-gnu.tar.gz && \
    tar -xzvf bitcoin-0.21.1-x86_64-linux-gnu.tar.gz && \
    install -m 0755 -o root -g root -t /usr/local/bin bitcoin-0.21.1/bin/* && \
    rm -rf bitcoin-0.21.1*

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
RUN go mod download

COPY ./arc_txt_endpoint_test.go ./arc_txt_endpoint_test.go
COPY ./utils.go ./utils.go

# This will compile and run the tests
CMD [ "go", "test", "-v" ,"./..."]
