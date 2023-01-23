#!/bin/bash

CGO=0

for i in "$@"
do
case $i in
    --force*)
    FORCE="true"
    shift # past argument=value
    ;;

    --cgo=*)
    CGO=1
    ;;

    --build=*)
    BUILD="${i#*=}"
    shift # past argument=value
    ;;

    *)
      # unknown option: ignore
    ;;
esac
done

cd $(dirname $BASH_SOURCE)

PROG_NAME=$(awk -F'"' '/^const progname =/ {print $2}' main.go)

if [ -z "$(git status --porcelain)" ]; then
  # Working directory clean
  GIT_COMMIT=$(git rev-parse HEAD)
  LAST_BUILD=$(cat build/commit.dat)
  if [[ $GIT_COMMIT == $LAST_BUILD ]]; then
    echo "$PROG_NAME: Nothing new."
    exit 1
  fi
elif [[ "$FORCE" == "true" ]]; then
  echo "Force build requested."
  GIT_COMMIT=$(date -u +%Y-%m-%d-%H-%M-%S)
else
  echo "${PROG_NAME}: Project must be clean before you can build"
  exit 1
fi

rm -rf build

echo "${PROG_NAME}: Building..."

#mkdir -p build/darwin
#mkdir -p build/windows
mkdir -p build/linux
#mkdir -p build/raspian

if [[ $BUILD == "" ]]; then
  FILENAME=${PROG_NAME}_${GIT_COMMIT}
else
  FILENAME=${PROG_NAME}_${GIT_COMMIT}_${BUILD}
fi

#go build --trimpath -o build/darwin/$FILENAME -ldflags="-X main.commit=${GIT_COMMIT} -X main.version=MANUAL"
env CGO_ENABLED=$CGO GOOS=linux GOARCH=amd64 go build --trimpath -o build/linux/$FILENAME -ldflags="-s -w -X main.commit=${GIT_COMMIT} -X main.version=MANUAL"
#env GOOS=linux GOARCH=arm go build --trimpath -o build/raspian/$FILENAME -ldflags="-s -w -X main.commit=${GIT_COMMIT} -X main.version=MANUAL"
#env GOOS=windows GOARCH=386 go build --trimpath -o build/windows/$FILENAME -ldflags="-s -w -X main.commit=${GIT_COMMIT} -X main.version=MANUAL"

# Now build any cmd programs and include them in the build artifact
for d in cmd/*; do
  if [ -d "${d}" ]; then
    if [ -f "${d}/.do_not_build" ]; then
      echo "Skipping ${d} as .do_not_build exists"
    else
      mkdir -p build/linux/cmd
      cd ${d}
      CMD=${d#cmd/}
      echo "Building ${CMD}"

      env CGO_ENABLED=$CGO GOOS=linux GOARCH=amd64 go build --trimpath -o ../../build/linux/cmd/${d##*/}
      if [ "$?" == "0" ]; then
        echo "Built ${d##*/}"
      else
        echo "Failed to build ${d##*/}"
      fi

      cd ../..
    fi
  fi
done

if [[ "$?" == "0" ]]; then
  echo $GIT_COMMIT > build/commit.dat
  echo "${PROG_NAME}: Built $FILENAME"

  cp settings_local.conf build/linux/
  cp -r assets build/linux/
  cd build/linux/
  tar cvfz ../../$FILENAME.tar.gz *
  echo "${PROG_NAME}: Artifact $FILENAME.tar.gz"
else
  echo "${PROG_NAME}: Build FAILED"
fi
