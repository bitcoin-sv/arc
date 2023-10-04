SHELL=/bin/bash

.PHONY: all
all: deps lint build test

.PHONY: deps
deps:
	go mod download

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: lint
lint:
	golangci-lint run -v ./...
	staticcheck ./...

.PHONY: run
run:
	sh run.sh

.PHONY: gen_go
gen_go:
	go generate ./...

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

coverage:
	rm -f ./cov.out
	go test -coverprofile=./cov.out ./...
.PHONY: clean
clean:
	rm -f ./arc_*.tar.gz
	rm -rf build/

.PHONY: install
install:
	# arch -arm64 brew install golangci-lint
	brew install pre-commit
	pre-commit install

.PHONY: install_gen
install_gen:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.15.0
	go install github.com/matryer/moq@v0.3.2

.PHONY: docs
docs:
	sh generate_docs.sh

gh-pages:
	git push --force origin `git subtree split --prefix doc master`:gh-pages

.PHONY: api
api:
	oapi-codegen -config api/config.yaml api/arc.yml > api/arc.go
