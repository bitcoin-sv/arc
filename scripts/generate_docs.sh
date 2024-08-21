#!/bin/bash

if ! [ -x "$(command -v widdershins)" ]; then
  npm install -g widdershins
fi

if ! [ -x "$(command -v swagger-cli)" ]; then
  npm install -g swagger-cli
fi

swagger-cli bundle -o pkg/api/arc.json pkg/api/arc.yaml
cp pkg/api/arc.json doc/
cp web/logo.png doc/dist/
widdershins --search false --language_tabs 'http:HTTP' 'javascript:JavaScript' 'java:Java' 'go:Go' 'ruby:Ruby' 'python:Python' 'shell:curl' --summary pkg/api/arc.json -o doc/api.md
