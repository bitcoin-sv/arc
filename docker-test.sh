#!/bin/bash

# Start the application in the background
./main &

# Wait for the application to start
sleep 5

# Run the tests
cd Test && go test -v