# Use an official Golang runtime as a parent image
FROM golang:1.19

# Set the working directory in the container
WORKDIR /app

# Add the rest of the code necessary for the app
ADD . /app

# Download all the dependencies
RUN go mod tidy

# Build the Go app
RUN go build -o main .

# Set environment variables for your application
# replace these with actual values
ENV arc_httpAddress=localhost:9090
ENV metamorph_grpcAddress=localhost:8001
ENV blocktx_grpcAddress=localhost:8011
ENV callbacker_grpcAddress=localhost:8021
ENV profilerAddr=localhost:9999

# Run the command on container startup
CMD ["./main"]
