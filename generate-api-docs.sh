#!/usr/bin/env bash

if ! [ -x "$(command -v widdershins)" ]; then
  npm install -g widdershins
fi

if ! [ -x "$(command -v shins)" ]; then
  npm install -g shins
fi

widdershins --search false --language_tabs 'http:HTTP' 'javascript:JavaScript' 'java:Java' 'go:Go' 'ruby:Ruby' 'python:Python' 'shell:curl' --summary mapi.json -o mapi.md
shins --inline --logo logo.png -o mapi.html mapi.md
