SHELL=/bin/bash

.PHONY: all
all: deps lint build test

.PHONY: deps
deps:
	go mod download

.PHONY: build
build:
	go build ./...

.PHONY: clean_e2e_tests
clean_e2e_tests:
	# Remove containers and images; avoid failure if the image doesn't exist
	docker container prune -f
	docker rmi test-tests || true


.PHONY: build_release
build_release:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o build/arc_linux_amd64 ./main.go

.PHONY: run_e2e_tests
run_e2e_tests:
	cd ./test && docker-compose up -d node1 node2 node3 arc
	cd ./test && docker-compose up --exit-code-from tests tests
	cd ./test && docker-compose down

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: install_lint
install_lint:
	go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: lint
lint:
	golangci-lint run -v ./...
	staticcheck ./...

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

.PHONY: coverage
coverage:
	rm -f ./cov.out
	go test -coverprofile=./cov.out -covermode=atomic ./... 2>&1 > gotest.out
	cat gotest.out | go-junit-report -set-exit-code > report.xml
	goverreport -coverprofile cov.out -packages -sort block

.PHONY: install_coverage
install_coverage:
	go install github.com/mcubik/goverreport@latest
	go install github.com/jstemmer/go-junit-report/v2@latest

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

.PHONY: clean_restart_e2e_test
clean_restart_e2e_test: clean_e2e_tests build_release run_e2e_tests

migrate_postgres:
	migrate -database "postgres://arcuser:arcpass@localhost:5432/arcdb?sslmode=disable"  -path database/migrations/postgres  up

migrate_sqlite:
	migrate -path database/migrations/sqlite -database "sqlite3://data/sqlite/arcdb.sqlite3"  up
