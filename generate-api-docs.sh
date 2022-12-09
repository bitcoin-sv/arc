#!/usr/bin/env bash

if ! [ -x "$(command -v widdershins)" ]; then
  npm install -g widdershins
fi

if ! [ -x "$(command -v shins)" ]; then
  npm install -g shins
fi

if ! [ -x "$(command -v swagger-cli)" ]; then
  npm install -g swagger-cli
fi

swagger-cli bundle -o arc.json arc.yml
widdershins --search false --language_tabs 'http:HTTP' 'javascript:JavaScript' 'java:Java' 'go:Go' 'ruby:Ruby' 'python:Python' 'shell:curl' --summary arc.json -o arc.md
shins --inline --logo logo.png -o arc.html arc.md
