
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
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags "-X $(REPOSITORY)/internal/version.Commit=$(APP_COMMIT) -X $(REPOSITORY)/internal/version.Version=$(APP_VERSION)" -o build/arc_linux_amd64 ./cmd/arc/main.go

.PHONY: build_docker
build_docker:
	docker build . -t test-arc --build-arg="APP_COMMIT=$(APP_COMMIT)" --build-arg="APP_VERSION=$(APP_VERSION)"

.PHONY: run
run:
	make build_docker
	docker compose --env-file ./.env.dev up blocktx callbacker metamorph api

.PHONY: run_e2e_tests
run_e2e_tests:
	make build_docker
	docker compose --env-file ./.env.dev up blocktx callbacker metamorph api tests --scale blocktx=2 --scale metamorph=2 --exit-code-from tests
	docker compose down

.PHONY: run_e2e_tests_with_tracing
run_e2e_tests_with_tracing:
	make build_docker
	ARC_TRACING_ENABLED=TRUE docker compose --env-file ./.env.dev up blocktx callbacker metamorph api tests jaeger --scale blocktx=2 --scale metamorph=2 --no-attach jaeger

.PHONY: run_e2e_mcast_tests
run_e2e_mcast_tests:
	make build_docker
	docker compose -f docker-compose-mcast.yaml --env-file ./.env.dev up mcast_sidecar blocktx metamorph api tests --scale blocktx=6 --exit-code-from tests
	docker compose -f docker-compose-mcast.yaml down

.PHONY: test
test:
	CGO_ENABLED=1 go test -ldflags="-w -s" -coverprofile=./cov.out -covermode=atomic -race -count=1 ./... -coverpkg ./...

.PHONY: test_short
test_short:
	go test -coverprofile=./cov_short.out -covermode=atomic -race -short -count=1 ./... -coverpkg ./...

.PHONY: coverage
coverage:
	go tool cover -html=cov.out -o coverage_report.html
	goverreport -coverprofile cov.out -packages -sort block

.PHONY: coverage_short
coverage_short:
	go tool cover -html=cov_short.out -o coverage_report_short.html
	goverreport -coverprofile cov_short.out -packages -sort block

.PHONY: install_lint
install_lint:
	go install honnef.co/go/tools/cmd/staticcheck@2025.1.1

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
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
	go install github.com/matryer/moq@v0.5.3

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
