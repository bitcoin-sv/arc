#!/bin/sh

cd $(dirname $BASH_SOURCE)

./liquibase.sh --database=localhost:15432 --cmd=clearchecksums