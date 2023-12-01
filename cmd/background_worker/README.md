# ARC background processing framework

## Overview
The goal of this submodule is to provide simple and convenient way to schedule repetitive tasks to be performed on ARC.
The current implementation contains only ARC tables clean up with frequency specified in configuration file.

## Key features
- easy configuration using config.yaml file
- no dependencies
- structured logs output using log/slog
- extendable layout for other scenarios

## System requirements
- go version `go1.21.1+`

## Usage

1. copy config.example.yaml to config.yaml
2. amend config.yaml parameters to reflect your database connection configuration, frequency and blocks expiration timeout
3. `go run main.go`
