#!/bin/bash

if ! [ -x "$(command -v widdershins)" ]; then
  npm install -g widdershins
fi

if ! [ -x "$(command -v shins)" ]; then
  npm install -g shins
fi

if ! [ -x "$(command -v swagger-cli)" ]; then
  npm install -g swagger-cli
fi

swagger-cli bundle -o api/arc.json api/arc.yml
cp api/arc.json doc/
widdershins --search false --language_tabs 'http:HTTP' 'javascript:JavaScript' 'java:Java' 'go:Go' 'ruby:Ruby' 'python:Python' 'shell:curl' --summary api/arc.json -o doc/api.md
