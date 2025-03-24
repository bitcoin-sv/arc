
REPOSITORY := github.com/bitcoin-sv/arc
APP_COMMIT := $(shell git rev-parse --short HEAD)
APP_VERSION := $(shell git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')

.PHONY: all
all: deps lint build test

.PHONY: deps
deps:
	go mod download

.PHONY: build
build:
	go build ./...

.PHONY: build_release
build_release:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X $(REPOSITORY)/internal/version.Commit=$(APP_COMMIT) -X $(REPOSITORY)/internal/version.Version=$(APP_VERSION)" -o build/arc_linux_amd64 ./cmd/arc/main.go

.PHONY: build_docker
build_docker:
	docker build . -t test-arc --build-arg="APP_COMMIT=$(APP_COMMIT)" --build-arg="APP_VERSION=$(APP_VERSION)"

.PHONY: run
run:
	docker compose down -v --remove-orphans
	docker compose up --build blocktx callbacker metamorph api

.PHONY: run_e2e_tests
run_e2e_tests:
	docker compose down -v --remove-orphans
	docker compose up --build blocktx callbacker metamorph api tests --scale blocktx=4 --scale metamorph=2 --exit-code-from tests
	docker compose down

.PHONY: run_e2e_tests_with_tracing
run_e2e_tests_with_tracing:
	docker compose down -v --remove-orphans
	ARC_TRACING_ENABLED=TRUE docker compose up --build blocktx callbacker metamorph api tests jaeger --scale blocktx=4 --scale metamorph=2 --no-attach jaeger

.PHONY: run_e2e_mcast_tests
run_e2e_mcast_tests:
	docker compose -f docker-compose-mcast.yaml down --remove-orphans
	docker compose -f docker-compose-mcast.yaml up --build mcast_sidecar blocktx metamorph api tests --scale blocktx=6 --exit-code-from tests
	docker compose -f docker-compose-mcast.yaml down

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: test_short
test_short:
	go test -race -short -count=1 ./...

.PHONY: install_lint
install_lint:
	go install honnef.co/go/tools/cmd/staticcheck@2024.1.1

.PHONY: lint
lint:
	golangci-lint run -v ./...
	staticcheck ./...

.PHONY: lint_fix
lint_fix:
	golangci-lint run -v ./... --fix

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
	internal/metamorph/metamorph_api/metamorph_api.proto

	protoc \
	--proto_path=. \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
	internal/blocktx/blocktx_api/blocktx_api.proto

	protoc \
	--proto_path=. \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
	internal/callbacker/callbacker_api/callbacker_api.proto

	protoc \
	--proto_path=. \
	--go_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_out=. \
	--go-grpc_opt=paths=source_relative \
	pkg/message_queue/nats/client/test_api/test_api.proto

.PHONY: clean_gen
clean_gen:
	rm -f ./internal/metamorph/metamorph_api/*.pb.go
	rm -f ./internal/blocktx/blocktx_api/*.pb.go
	rm -f ./internal/callbacker/callbacker_api/*.pb.go

.PHONY: coverage
coverage:
	rm -f ./cov.out
	go test -coverprofile=./cov.out -covermode=atomic  ./... 2>&1 > gotest.out
	go tool cover -html=cov.out -o coverage-report.html
	goverreport -coverprofile cov.out -packages -sort block

.PHONY: install_coverage
install_coverage:
	go install github.com/mcubik/goverreport@latest
	go install github.com/jstemmer/go-junit-report/v2@latest

.PHONY: clean
clean:
	rm -rf build/

.PHONY: install
install:
	# arch -arm64 brew install golangci-lint
	brew install pre-commit
	pre-commit install
	pre-commit install --hook-type commit-msg

.PHONY: install_gen
install_gen:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
	go install github.com/matryer/moq@latest

.PHONY: docs
docs:
	sh scripts/generate_docs.sh

gh-pages:
	git push --force origin `git subtree split --prefix doc master`:gh-pages

.PHONY: api
api:
	oapi-codegen -config pkg/api/config.yaml pkg/api/arc.yaml > pkg/api/arc.go

.PHONY: compare_config
compare_config:
	rm -f ./config/dumped_config.yaml
	go run ./cmd/arc/main.go -dump_config "./config/dumped_config.yaml" && go run ./scripts/compare_yamls.go
	rm ./config/dumped_config.yaml
