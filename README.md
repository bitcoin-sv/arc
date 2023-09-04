# ARC

## Documentation

- Find full documentation at [https://bitcoin-sv.github.io/arc](https://bitcoin-sv.github.io/arc)

## Microservices

To run all the microservices in one process (during development), use the `main.go` file in the root directory.

```shell
go run main.go
```dd

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
```

NOTE: If you start the `main.go` with a microservice set to true, it will not start the other services. For example, if
you run `go run main.go -api=true`, it will only start the API server, and not the other services, although you can start multiple services by specifying them on the command line.

### API

API is the REST API microservice for interacting with ARC. See the [API documentation](/doc/api.html) for more information.

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

| Code | Status                 | Description                                                                                                                                                                                                    |
|-----|------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 0   | `UNKNOWN`              | The transaction has been sent to metamorph, but no processing has taken place. This should never be the case, unless something goes wrong.                                                                     |
| 1   | `QUEUED`               | The transaction has been queued for processing.                                                                                                                                                                |
| 2   | `RECEIVED`             | The transaction has been properly received by the metamorph processor.                                                                                                                                         |
| 3   | `STORED`               | The transaction has been stored in the metamorph store. This should ensure the transaction will be processed and retried if not picked up immediately by a mining node.                                        |
| 4   | `ANNOUNCED_TO_NETWORK` | The transaction has been announced (INV message) to the Bitcoin network.                                                                                                                                       |
| 5   | `REQUESTED_BY_NETWORK` | The transaction has been requested from metamorph by a Bitcoin node.                                                                                                                                           |
| 6   | `SENT_TO_NETWORK`      | The transaction has been sent to at least 1 Bitcoin node.                                                                                                                                                      |
| 7   | `ACCEPTED_BY_NETWORK`  | The transaction has been accepted by a connected Bitcoin node on the ZMQ interface. If metamorph is not connected to ZQM, this status will never by set.                                                       |
| 8   | `SEEN_ON_NETWORK`      | The transaction has been seen on the Bitcoin network and propagated to other nodes. This status is set when metamorph receives an INV message for the transaction from another node than it was sent to.       |
| 9   | `MINED`                | The transaction has been mined into a block by a mining node.                                                                                                                                                  |
| 108 | `CONFIRMED`            | The transaction is marked as confirmed when it is in a block with 100 blocks built on top of that block.                                                                                                       |
| 109 | `REJECTED`             | The transaction has been rejected by the Bitcoin network.                                                                                                                                                      |

This status is returned in the `txStatus` field whenever the transaction is queried.

#### Metamorph stores

The metamorph store has been implemented for multiple databases, depending on your needs. In high-volume environments,
you may want to use a database that is optimized for high throughput, such as [Badger](https://dgraph.io/docs/badger).

The following databases have been implemented:

* Sqlite3 (`sqlite` or `sqlite_memory` for in-memory)
* Postgres (`postgres`)
* Badger (`badger`)
* BadgerHold (`badgerhold`)

You can select the store to use by setting the `metamorph_dbMode` in the settings file or adding `metamorph_dbMode` as
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

If you want to use zmq, you can set the `peer_%d_zmq` settings to the full url
(example `zmq://bitcoin:bitcoin@localhost:18332`) of the zmq endpoints in the settings file.

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

The BlockTx store has been implemented for multiple databases, depending on your needs. In high-volume environments,
you may want to use a database that is optimized for high throughput, such as Postgres.

The following databases have been implemented:

* Sqlite3 (`sqlite` or `sqlite_memory` for in-memory)
* Postgres (`postgres`)

You can select the store to use by setting the `blocktx_dbMode` in the settings file or adding `blocktx_dbMode` as
an environment variable.

Please note that if you are running multiple instances of BlockTX for resilience, each BlockTx can be configured to use a shared database and in this case, Postgres is probably a sensible choice.

### Callbacker

Callbacker is a very simple microservice that is responsible for sending callbacks to clients when a transaction has
been accepted by the Bitcoin network. To register a callback, the client must add the `X-CallbackUrl` header to the
request. The callbacker will then send a POST request to the URL specified in the header, with the transaction ID in
the body. See the [API documentation](#API) for more information.

You can run the Callbacker service like this:

```shell
go run cmd/callbacker/main.go
```

or using the generic `main.go`:

```shell
go run main.go -callbacker=true
```
