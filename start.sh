#!/bin/sh

# Start the arc service in the background
/service/arc &

# Wait for arc to become healthy
echo "Waiting for arc to be healthy..."
until curl -s http://localhost:9090/healthz | grep "ok"; do
  sleep 5
  echo "Waiting for arc to start..."
done

# Start the broadcaster-cli service
/service/broadcaster-cli keyset balance