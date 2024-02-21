# ARC
ARC is a transaction processor for Bitcoin that keeps track of the life cycle of a transaction as it is processed by the Bitcoin network. Next to the mining status of a transaction, ARC also keeps track of the various states that a transaction can be in.

## Documentation

- Find full documentation at [https://bitcoin-sv.github.io/arc](https://bitcoin-sv.github.io/arc)

## Configuration
Settings for ARC are defined in a configuration file. The default configuration file is `config.yaml` in the root directory. Each setting is documented in the file itself.
If you want to load `config.yaml` from a different location, you can specify it on the command line using the `-config=<path>` flag.

Each setting in the file `config.yaml` can be overridden with an environment variable. The environment variable needs to have this prefix `ARC_`. A sub setting will be separated using an underscore character. For example the following config setting could be overridden by the environment variable `ARC_METAMORPH_LISTENADDR`
```yaml
metamorph:
  listenAddr:
```

## Microservices

To run all the microservices in one process (during development), use the `main.go` file in the root directory.

```shell
go run main.go
```

The `main.go` file accepts the following flags (`main.go --help`):

```shell
usage: main [options]
where options are:

    -api=<true|false>
          whether to start ARC api server (default=true)

    -metamorph=<true|false>
          whether to start metamorph (default=true)

    -blocktx=<true|false>
          whether to start block tx (default=true)

    -callbacker=<true|false>
          whether to start callbacker (default=true)

    -tracer=<true|false>
          whether to start the Jaeger tracer (default=false)

    -config=<path>
          path to config file (default='')
```

NOTE: If you start the `main.go` with a microservice set to true, it will not start the other services. For example, if
you run `go run main.go -api=true`, it will only start the API server, and not the other services, although you can start multiple services by specifying them on the command line.

### API

API is the REST API microservice for interacting with ARC. See the [API documentation](https://bitcoin-sv.github.io/arc/api.html) for more information.

The API takes care of authentication, validation, and sending transactions to Metamorph.  The API talks to one or more Metamorph instances using client-based, round robin load balancing.

You can run the API like this:

```shell
go run cmd/api/main.go
```

or using the generic `main.go`:

```shell
go run main.go -api=true
```

The only difference between the two is that the generic `main.go` starts the Go profiler, while the specific `cmd/api/main.go`
command does not.

#### Integration into an echo server

If you want to integrate the ARC API into an existing echo server, check out the
[`examples`](https://github.com/bitcoin-sv/arc/tree/master/examples) folder in the GitHub repo.

A very simple example:

```go
package main

import (
	"fmt"

	"github.com/bitcoin-sv/arc/api"
	apiHandler "github.com/bitcoin-sv/arc/api/handler"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/labstack/echo/v4"
)

func main() {

	// Set up a basic Echo router
	e := echo.New()

	// add a single bitcoin node
	txHandler, err := transactionHandler.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		panic(err)
	}

	// initialise the arc default api handler, with our txHandler and any handler options
	var handler api.HandlerInterface
	if handler, err = apiHandler.NewDefault(txHandler); err != nil {
		panic(err)
	}

	// Register the ARC API
	// the arc handler registers routes under /v1/...
	api.RegisterHandlers(e, handler)
	// or with a base url => /mySubDir/v1/...
	// arc.RegisterHandlersWithBaseURL(e. blocktx_api, "/arc")

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", "0.0.0.0", 8080)))
}
```

This will initialise the ARC API with a single Bitcoin node (not Metamorph), similar to how MAPI is run at the moment.

### Metamorph

Metamorph is a microservice that is responsible for processing transactions sent by the API to the Bitcoin network. It
takes care of re-sending transactions if they are not acknowledged by the network within a certain time period (60
seconds by default).

Metamorph is designed to be horizontally scalable, with each instance operating independently and having its own
transaction store. As a result, they do not communicate with each other and remain unaware of each other's existence.

You can run metamorph like this:

```shell
go run cmd/metamorph/main.go
```

or using the generic `main.go`:

```shell
go run main.go -metamorph=true
```

The only difference between the two is that the generic `main.go` starts the Go profiler, while the specific
`cmd/metamorph/main.go` command does not.

#### Metamorph transaction statuses

Metamorph keeps track of the lifecycle of a transaction, and assigns it a status. The following statuses are
available:

| Code | Status                 | Description                                                                                                                                                                                              |
|-----|------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 0   | `UNKNOWN`              | The transaction has been sent to metamorph, but no processing has taken place. This should never be the case, unless something goes wrong.                                                               |
| 1   | `QUEUED`               | The transaction has been queued for processing.                                                                                                                                                          |
| 2   | `RECEIVED`             | The transaction has been properly received by the metamorph processor.                                                                                                                                   |
| 3   | `STORED`               | The transaction has been stored in the metamorph store. This should ensure the transaction will be processed and retried if not picked up immediately by a mining node.                                  |
| 4   | `ANNOUNCED_TO_NETWORK` | The transaction has been announced (INV message) to the Bitcoin network.                                                                                                                                 |
| 5   | `REQUESTED_BY_NETWORK` | The transaction has been requested from metamorph by a Bitcoin node.                                                                                                                                     |
| 6   | `SENT_TO_NETWORK`      | The transaction has been sent to at least 1 Bitcoin node.                                                                                                                                                |
| 7   | `ACCEPTED_BY_NETWORK`  | The transaction has been accepted by a connected Bitcoin node on the ZMQ interface. If metamorph is not connected to ZMQ, this status will never by set.                                                 |
| 8   | `SEEN_ON_NETWORK`      | The transaction has been seen on the Bitcoin network and propagated to other nodes. This status is set when metamorph receives an INV message for the transaction from another node than it was sent to. |
| 9   | `MINED`                | The transaction has been mined into a block by a mining node.                                                                                                                                            |
| 10  | `SEEN_IN_ORPHAN_MEMPOOL`             | The transaction has been sent to at least 1 Bitcoin node but parent transaction was not found. |
| 108 | `CONFIRMED`            | The transaction is marked as confirmed when it is in a block with 100 blocks built on top of that block.                                                                                                 |
| 109 | `REJECTED`             | The transaction has been rejected by the Bitcoin network.                                                                                                                                                |

This status is returned in the `txStatus` field whenever the transaction is queried.

#### Metamorph stores

Currently, metamorph only offers one storage implementation which is Postgres.
~~The metamorph store has been implemented for multiple databases, depending on your needs. In high-volume environments,
you may want to use a database that is optimized for high throughput, such as [Badger](https://dgraph.io/docs/badger).~~

The following databases have been implemented:

* Postgres (`postgres`)
* ~~Sqlite3 (`sqlite` or `sqlite_memory` for in-memory)~~
* ~~Badger (`badger`)~~
* ~~BadgerHold (`badgerhold`)~~

You can select the store to use by setting the `metamorph.db.mode` in the settings file or adding `metamorph.db.mode` as
an environment variable.

#### Connections to Bitcoin nodes

Metamorph can connect to multiple Bitcoin nodes, and will use a subset of the nodes to send transactions to. The other
nodes will be used to listen for transaction INV message, which will trigger the SEEN_ON_NETWORK status of a transaction.

The Bitcoin nodes can be configured in the settings file.

#### Whitelisting

Metamorph is talking to the Bitcoin nodes over the p2p network. If metamorph sends invalid transactions to the
Bitcoin node, it will be **banned** by that node. Either make sure not to send invalid or double spend transactions through
metamorph, or make sure that all metamorph servers are **whitelisted** on the Bitcoin nodes they are connecting to.

#### ZMQ

Although not required, zmq can be used to listen for transaction messages (`hashtx`, `invalidtx`, `discardedfrommempool`).
This is especially useful if you are not connecting to multiple Bitcoin nodes, and therefore are not receiving INV
messages for your transactions.

If you want to use zmq, you can set the `host.port.zmq` setting for the respective `peers` setting in the configuration file.

ZMQ does seem to be a bit faster than the p2p network, so it is recommended to turn it on, if available.

### BlockTx

BlockTx is a microservice that is responsible for processing blocks mined on the Bitcoin network, and for propagating
the status of transactions to each Metamorph that has subscribed to this service.

The main purpose of BlockTx is to de-duplicate processing of (large) blocks. As an incoming block is processed by BlockTx, each Metamorph is notified of transactions that they have registered an interest in.  BlockTx does not store the transaction data, but instead stores only the transaction IDs and the block height in which
they were mined. Metamorph is responsible for storing the transaction data.

You can run BlockTx like this:

```shell
go run cmd/blocktx/main.go
```

or using the generic `main.go`:

```shell
go run main.go -blocktx=true
```

The only difference between the two is that the generic `main.go` starts the Go profiler, while the specific
`cmd/blocktx/main.go`command does not.

#### BlockTx stores

Currently, metamorph only offers one storage implementation which is Postgres.
~~The BlockTx store has been implemented for multiple databases, depending on your needs. In high-volume environments,
you may want to use a database that is optimized for high throughput, such as Postgres.~~

The following databases have been implemented:

* Postgres (`postgres`)
* ~~Sqlite3 (`sqlite` or `sqlite_memory` for in-memory)~~

You can select the store to use by setting the `blocktx.db.mode` in the settings file or adding `blocktx.db.mode` as
an environment variable.

Please note that if you are running multiple instances of BlockTX for resilience, each BlockTx can be configured to use a shared database and in this case, Postgres is probably a sensible choice.

If BlockTx is configured to run with `postgres` db, then migrations have to be executed prior to starting ARC. For this you'll need the [go-migrate](https://github.com/golang-migrate/migrate) tool. Once `go-migrate` has been installed, the migrations can be executed as follows:
```bash
migrate -database "postgres://<username>:<password>@<host>:<port>/<db-name>?sslmode=<ssl-mode>"  -path database/migrations/postgres  up
```

### Callbacks

To register a callback, the client must add the `X-CallbackUrl` header to the
request. The callbacker will then send a POST request to the URL specified in the header, with the transaction ID in
the body. See the [API documentation](https://bitcoin-sv.github.io/arc/api.html) for more information.

### K8s-Watcher

The K8s-Watcher is a service which is needed for a special use case. If ARC runs on a Kubernetes cluster and is configured to run with AWS DynamoDB as a `metamorph` centralised storage, then the K8s-Watcher can be run as a safety measure. Due to the centralisation of `metamorph` storage, each `metamorph` pod has to ensure the exclusive processing of records by locking the records. If `metamorph` shuts down gracefully it will unlock all the records it holds in memory. The graceful shutdown is not guaranteed though. For this eventuality the K8s-Watcher can be run in a separate pod. K8s-Watcher detects when `metamorph` pods are terminated and will additionally call on the `metamorph` service to unlock the records of that terminated `metamorph` pod. This ensures that no records will stay in a locked state.

The K8s-Watcher can be started as follows

```shell
go run main.go -k8s-watcher=true
```

## Broadcaster

Broadcaster is a tool to broadcast example transactions to ARC. It can be used to test the ARC API and Metamorph.

Examples of broadcaster usage:
```bash
# Send 10 txs via API and send back to the original address (consolidate). Address for API is configured in config.yaml - broadcaster.apiURL
go run cmd/broadcaster/main.go -api=true -consolidate -keyfile=./cmd/broadcaster/arc.key -authorization=mainnet_XXX 10

# Send 20 txs with 5 txs/batch to a single metamorph via gRPC. Address for metamorph is configured in config.yaml - metamorph.dialAddr
go run cmd/broadcaster/main.go -api=false -consolidate -keyfile=./cmd/broadcaster/arc.key -authorization=mainnet_XXX -batch=5 20
```

Detailed information about flags can is displayed by running `go run cmd/broadcaster/main.go`.

## Background worker

The goal of this submodule is to provide simple and convenient way to schedule repetitive tasks to be performed on ARC.

## Tests
### Unit tests
In order to run the unit tests do the following
```
make test
```

### Integration tests
Integration tests of DynamoDB need docker installed to run them. If `colima` implementation of Docker is being used on macOS, the `DOCKER_HOST` environment variable may need to be given as follows
```bash
DOCKER_HOST=unix:///Users/<username>/.colima/default/docker.sock make test
```
These integration tests can be excluded from execution with `go test ./...` by adding the `-short` flag like this `go test -short ./...`.

### end-to-end tests
The end-to-end tests are located in the folder `test`. Docker needs to be installed in order to run them. End-to-end tests can be run locally together with arc and 3 nodes using the provided docker-compose file.
The tests can be executed like this:
```
make clean_restart_e2e_test
```

## Profiler
Each service runs a http profiler server if it is configured in `config.yaml`. In order to access it, a connection can be created using the Go `pprof` [tool](https://pkg.go.dev/net/http/pprof). For example to investigate the memory usage
```bash
go tool pprof http://localhost:9999/debug/pprof/allocs
```
Then type `top` to see the functions which consume the most memory. Find more information [here](https://go.dev/blog/pprof).
