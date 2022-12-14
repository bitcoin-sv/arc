#!/bin/sh

DB="localhost:15432"

for i in "$@"
do
case $i in
    --database=*)
    DB="${i#*=}"
    shift # past argument=value
    ;;

    --cmd=*)
    CMD="${i#*=}"
    shift # past argument=value
    ;;

    *)
    # unknown option, ignore
    ;;
esac
done

cd $(dirname $BASH_SOURCE)

./liquibase-3.4.0/liquibase \
  --url="jdbc:postgresql://$DB/blocktx" \
  --username=blocktx \
  --password=blocktx \
  --logLevel=info \
  --changeLogFile="patches/root.xml" \
  $CMD
