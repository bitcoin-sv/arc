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
	go test -race -v -count=1 ./...

.PHONY: lint
lint:
	golangci-lint run --skip-dirs p2p/wire
	staticcheck ./...

.PHONY: run
run:
	sh run.sh

.PHONY: gen
gen:
	protoc \
	--proto_path=. \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
	metamorph/metamorph_api/metamorph_api.proto

	protoc \
	--proto_path=. \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
	blocktx/blocktx_api/blocktx_api.proto

	protoc \
	--proto_path=. \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
	callbacker/callbacker_api/callbacker_api.proto

.PHONY: clean_gen
clean_gen:
	rm -f ./metamorph/metamorph_api/*.pb.go
	rm -f ./blocktx/blocktx_api/*.pb.go
	rm -f ./callbacker/callbacker_api/*.pb.go

.PHONY: clean
clean:
	rm -f ./arc_*.tar.gz
	rm -rf build/

.PHONY: install
install:
	# arch -arm64 brew install golangci-lint
	brew install pre-commit
	pre-commit install
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	# GO111MODULE=off go get -u github.com/mwitkow/go-proto-validators/protoc-gen-govalidators

.PHONY: docs
docs:
	sh generate_docs.sh

.PHONY: api
api:
	sh generate_api.sh
