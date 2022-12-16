SHELL=/bin/bash

.PHONY: all
all: deps lint build test

.PHONY: deps
deps:
	go mod download

.PHONY: build
build:
	sh build.sh

.PHONY: test
test:
	go test -v -count=1 ./...

.PHONY: lint
lint:
	golangci-lint run --skip-dirs p2p/wire

.PHONY: run
run:
	sh run.sh

.PHONY: gen
gen:
	protoc \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
    metamorph/api/metamorph_api.proto

	protoc \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
    blocktx/api/blocktx_api.proto

.PHONY: clean_gen
clean_gen:
	rm -f ./metamorph/api/*.pb.go
	rm -f ./blocktx/api/*.pb.go

.PHONY: clean
clean:
	rm -f ./arc_*.tar.gz
	rm -rf build/
 
.PHONY: install
install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

.PHONY: docs
docs:
	sh generate_docs.sh

.PHONY: api
api:
	sh generate_api.sh
