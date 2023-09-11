# Start from a Debian base image with ARM hard-float libraries support
FROM debian:bullseye-slim

# Set the Current Working Directory inside the container
WORKDIR /app

# Install required packages
RUN apt-get update && apt-get install -y wget tar netcat-openbsd gcc g++ make

# Download and install Go
ENV GOLANG_VERSION 1.17
RUN wget https://dl.google.com/go/go$GOLANG_VERSION.linux-arm64.tar.gz && \
    tar -C /usr/local -xzf go$GOLANG_VERSION.linux-arm64.tar.gz && \
    rm go$GOLANG_VERSION.linux-arm64.tar.gz

# Add Go binaries to PATH
ENV PATH $PATH:/usr/local/go/bin

# Download and install Bitcoin CLI for ARM
RUN wget https://bitcoincore.org/bin/bitcoin-core-0.21.1/bitcoin-0.21.1-arm-linux-gnueabihf.tar.gz && \
    tar -xzvf bitcoin-0.21.1-arm-linux-gnueabihf.tar.gz && \
    install -m 0755 -o root -g root -t /usr/local/bin bitcoin-0.21.1/bin/* && \
    rm -rf bitcoin-0.21.1* 

# Install the ARM hard-float libraries
RUN dpkg --add-architecture armhf && \
    apt-get update && \
    apt-get install -y libc6:armhf

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and the go.sum files are not changed
RUN go mod download

# Copy everything from the current directory to the Working Directory inside the container
COPY . .

# Copy the script into the container
COPY wait-for-it.sh /wait-for-it.sh
RUN chmod +x /wait-for-it.sh

# This will compile and run the tests
CMD [ "go", "test", "./..."]
