#!/bin/sh

cd $(dirname $BASH_SOURCE)

./liquibase.sh --database=localhost:5432 --cmd=update