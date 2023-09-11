#!/bin/bash

set -e

host="$1"
shift
cmd="$@"

until nc -z -v -w30 $host 9090
do
  echo "Waiting for the server to start..."
  # wait for 5 seconds before checking again
  sleep 5
done

>&2 echo "Server is up - executing command"

exec $cmd
