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
EXPOSE 9090

# Run the command on container startup
CMD ["./main"]