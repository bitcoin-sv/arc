# ARC
ARC is a transaction processor for Bitcoin that keeps track of the life cycle of a transaction as it is processed by the Bitcoin network. Next to the mining status of a transaction, ARC also keeps track of the various states that a transaction can be in.

## Documentation

- Find full documentation at [https://bitcoin-sv.github.io/arc](https://bitcoin-sv.github.io/arc)

## Configuration
Settings for ARC are defined in a configuration file. The default configuration file is `config/config.yaml`. Each setting is documented in the file itself.
If you want to load `config.yaml` from a different location, you can specify it on the command line using the `-config=<path>` flag.

Each setting in the file `config.yaml` can be overridden with an environment variable. The environment variable needs to have this prefix `ARC_`. A sub setting will be separated using an underscore character. For example the following config setting could be overridden by the environment variable `ARC_METAMORPH_LISTENADDR`
```yaml
metamorph:
  listenAddr:
```


## How to run ARC

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

    -config=<path>
          path to config file (default='')
```

Each individual microservice can also be started individually by running e.g. `go run main.go -api=true`.
NOTE: If you start the `main.go` with a microservice set to true, it will not start the other services. For example, if
you run `go run main.go -api=true`, it will only start the API server, and not the other services, although you can start multiple services by specifying them on the command line.

In order to run ARC there needs to be a Postgres database available. The connection to the database is defined in the `config.yaml` file. The database needs to be created before running ARC. The migrations for the database can be found in the `internal/metamorph/store/postgresql/migrations` folder. The migrations can be executed using the [go-migrate](https://github.com/golang-migrate/migrate) tool (see section [Metamorph stores](#metamorph-stores) and [Blocktx stores](#blocktx-stores)).

Additionally ARC relies on a message queue to communicate between Metamorph and BlockTx (see section [Message Queue](#message-queue)) section. The message queue can be started as a docker container. The docker image can be found [here](https://hub.docker.com/_/nats). The message queue can be started like this:

```
docker run -p 4222:4222 nats
```

The [docker-compose file](./test/docker-compose.yml) of the e2e-tests (see section [E2E tests](#e2e-tests)) additionally shows how ARC can be run with the message queue and the Postgres database and db migrations.

### Docker
ARC can be run as a docker container. The docker image can be built using the provided `Dockerfile` (see section [Building ARC](#building-arc)).

The latest docker image of ARC can be found [here](https://hub.docker.com/r/bsvb/arc/).

## Microservices

### API

API is the REST API microservice for interacting with ARC. See the [API documentation](https://bitcoin-sv.github.io/arc/api.html) for more information.

The API takes care of authentication, validation, and sending transactions to Metamorph.  The API talks to one or more Metamorph instances using client-based, round robin load balancing.

To register a callback, the client must add the `X-CallbackUrl` header to the
request. The callbacker will then send a POST request to the URL specified in the header, with the transaction ID in
the body. See the [API documentation](https://bitcoin-sv.github.io/arc/api.html) for more information.

You can run the API like this:

```shell
go run main.go -api=true
```

The only difference between the two is that the generic `main.go` starts the Go profiler, while the specific `cmd/api/main.go`
command does not.

#### Integration into an echo server

If you want to integrate the ARC API into an existing echo server, check out the
[examples](./examples) folder in the GitHub repo.

### Metamorph

Metamorph is a microservice that is responsible for processing transactions sent by the API to the Bitcoin network. It
takes care of re-sending transactions if they are not acknowledged by the network within a certain time period (60
seconds by default).

Metamorph is designed to be horizontally scalable, with each instance operating independently. As a result, they do not communicate with each other and remain unaware of each other's existence.

You can run metamorph like this:

```shell
go run main.go -metamorph=true
```

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
| 108 | `CONFIRMED`            | The transaction is marked as confirmed when it is in a block with 100 blocks built on top of that block (Currently this status is not maintained)                                                                                                 |
| 109 | `REJECTED`             | The transaction has been rejected by the Bitcoin network.                                                                                                                                                |

This status is returned in the `txStatus` field whenever the transaction is queried.

#### Metamorph stores

Currently, Metamorph only offers one storage implementation which is Postgres.

Migrations have to be executed prior to starting Metamorph. For this you'll need the [go-migrate](https://github.com/golang-migrate/migrate) tool. Once `go-migrate` has been installed, the migrations can be executed as follows:
```bash
migrate -database "postgres://<username>:<password>@<host>:<port>/<db-name>?sslmode=<ssl-mode>"  -path internal/metamorph/store/postgresql/migrations  up
```

#### Connections to Bitcoin nodes

Metamorph can connect to multiple Bitcoin nodes, and will use a subset of the nodes to send transactions to. The other
nodes will be used to listen for transaction **INV** message, which will trigger the SEEN_ON_NETWORK status of a transaction.

The Bitcoin nodes can be configured in the settings file.

#### Whitelisting

Metamorph is talking to the Bitcoin nodes over the p2p network. If metamorph sends invalid transactions to the
Bitcoin node, it will be **banned** by that node. Either make sure not to send invalid or double spend transactions through
metamorph, or make sure that all metamorph servers are **whitelisted** on the Bitcoin nodes they are connecting to.

#### ZMQ

Although not required, zmq can be used to listen for transaction messages (`hashtx`, `invalidtx`, `discardedfrommempool`).
This is especially useful if you are not connecting to multiple Bitcoin nodes, and therefore are not receiving INV
messages for your transactions. Currently, ARC can only detect whether a transaction was rejected e.g. due to double spending if ZMQ is connected to at least one node.

If you want to use zmq, you can set the `host.port.zmq` setting for the respective `peers` setting in the configuration file.

ZMQ does seem to be a bit faster than the p2p network, so it is recommended to turn it on, if available.

### BlockTx

BlockTx is a microservice that is responsible for processing blocks mined on the Bitcoin network, and for propagating
the status of transactions to Metamorph. The communication between BlockTx and Metamorph is asynchronous and happens through a message queue. More details about that message queue can be found [here](#message-queue-).

The main purpose of BlockTx is to de-duplicate processing of (large) blocks. As an incoming block is processed by BlockTx, each Metamorph is notified of transactions that they have registered an interest in.  BlockTx does not store the transaction data, but instead stores only the transaction IDs and the block height in which
they were mined. Metamorph is responsible for storing the transaction data.

You can run BlockTx like this:

```shell
go run main.go -blocktx=true
```

#### BlockTx stores

Currently, BlockTx only offers one storage implementation which is Postgres.

Migrations have to be executed prior to starting BlockTx. For this you'll need the [go-migrate](https://github.com/golang-migrate/migrate) tool. Once `go-migrate` has been installed, the migrations can be executed as follows:
```bash
migrate -database "postgres://<username>:<password>@<host>:<port>/<db-name>?sslmode=<ssl-mode>"  -path internal/blocktx/store/postgresql/migrations  up
```

<a id="message-queue"></a>
## Message Queue

For the communication between Metamorph and BlockTx a message queue is used. Currently the only available implementation of that message queue uses [NATS](https://nats.io/). A message queue of this type has to run in order for ARC to run.

Metamorph publishes new transactions to the message queue and BlockTx subscribes to the message queue, receive the transactions and stores them. Once BlockTx finds these transactions have been mined in a block it updates the block information and publishes the block information to the message queue. Metamorph subscribes to the message queue and receives the block information and updates the status of the transactions.

![Message Queue](./doc/message_queue.png)


## K8s-Watcher

The K8s-Watcher is a service which is needed for a special use case. If ARC runs on a Kubernetes cluster and is configured to run with AWS DynamoDB as a `metamorph` centralised storage, then the K8s-Watcher can be run as a safety measure. Due to the centralisation of `metamorph` storage, each `metamorph` pod has to ensure the exclusive processing of records by locking the records. If `metamorph` shuts down gracefully it will unlock all the records it holds in memory. The graceful shutdown is not guaranteed though. For this eventuality the K8s-Watcher can be run in a separate pod. K8s-Watcher detects when `metamorph` pods are terminated and will additionally call on the `metamorph` service to unlock the records of that terminated `metamorph` pod. This ensures that no records will stay in a locked state.

The K8s-Watcher can be started as follows

```shell
go run main.go -k8s-watcher=true
```

## Broadcaster-cli

The `broadcaster-cli` provides a set of functions which allow to interact with any instance of ARC. It also provides functions for key sets.

### Installation

The broadcaster-cli can be installed using the following command.
```
go install github.com/bitcoin-sv/arc/cmd/broadcaster-cli@latest
```

If the ARC repository is checked out it can also be installed from that local repository like this
```
go install ./cmd/broadcaster-cli/
```

### Configuration

`broadcaster-cli` uses flags for adding context needed to run it. The flags and commands available can be shown by running `broadcaster-cli` with the flag `--help`.

As there can be a lot of flags you can also define them in a `.env` file. For example like this:
```
keyfile=./cmd/broadcaster-cli/arc-0.key
testnet=true
```
If file `.env` is present in either the folder where `broadcaster-cli` is run or the folder `./cmd/broadcaster-cli/`, then these values will be used as flags (if available to the command). You can still provide the flags, in that case the value provided in the flag will override the value provided in `.env`

### How to use broadcaster-cli to send batches of transactions to ARC

These instructions will provide the steps needed in order to use `broadcaster-cli` to send transactions to ARC.

1. Create a new key set by running `broadcaster-cli keyset new`
   1. You can give a path where the key set should be stored as a file using the `--filename` flag
   2. Existing files will not be overwritten
   3. Omitting the `--filename` flag will create the file using a file name `./cmd/broadcaster-cli/arc-{i}.key`, where i is an iterator counting up until an available filename is found
2. The keyfile flag `--keyfile=<path to key file>` and `--testnet` flag have to be given in all commands except `broadcaster-cli keyfile new`
3. Add funds to the funding address
   1. Show the funding address by running `broadcaster-cli keyset address`
   2. In case of `testnet` (using the `--testnet` flag) funds can be added using the WoC faucet. For that you can use the command `broadcaster-cli keyset topup --testnet`
   3. You can view the balance of the key set using the command `broadcaster-cli keyset balance`
4. Create utxo set
   1. There must be a certain utxo set available so that `broadcaster-cli` can broadcast a reasonable number of transactions in batches
   2. First look at the existing utxo set using `broadcaster-cli keyset utxos`
   3. In order to create more outputs use the following command `broadcaster-cli utxos create --outputs=<number of outputs> --satoshis=<number of satoshis per output>`
   4. This command will send transactions creating the requested outputs to ARC. There are more flags needed for this command. Please see `go run cmd/broadcaster-cli/main.go utxos -h` for more details
   5. See the new distribution of utxos using `broadcaster-cli keyset utxos`
5. Broadcast transactions to ARC
   1. Now `broadcaster-cli` can be used to broadcast transactions to ARC at a given rate using this command `broadcaster-cli utxos broadcast --rate=<txs per second> --batchsize=<nr ot txs per batch>`
   2. The limit flag `--limit=<nr of transactions at which broadcasting stops>` is optional. If not given `broadcaster-cli` will only stop at abortion e.g. using `CTRL+C`
   3. The optional `--store` flag will store all the responses of each request to ARC in a folder `results/` as a json file
   4. In order to broadcast a large number of transactions in parallel, multiple key sets can be given in a comma separated way using the keyfile flag `--keyfile=./cmd/broadcaster-cli/arc-0.key,./cmd/broadcaster-cli/arc-1.key,./cmd/broadcaster-cli/arc-2.key`
      1. Each concurrently running broadcasting process will broadcast at the given rate
      2. For example: If a rate of `--rate=100` is given with 3 key files `--keyfile=arc-1.key,arc-2.key,arc-3.key`, then the final rate will be 300 transactions per second.
6. Consolidate outputs
   1. If not enough outputs are available for another test run it is best to consolidate the outputs so that there remains only output using `broadcaster-cli utxos consolidate`
   2. After this step you can continue with step 4
      1. Before continuing with step 4 it is advisable to wait until all consolidation transactions were mined
      2. The command `broadcaster-cli keyset balance` shows the amount of satoshis in the balance that have been confirmed and the amount which has not yet been confirmed

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

### E2E tests
The end-to-end tests are located in the folder `test`. Docker needs to be installed in order to run them. End-to-end tests can be run locally together with arc and 3 nodes using the provided docker-compose file.
The tests can be executed like this:
```
make clean_restart_e2e_test
```

The [docker-compose](./test/docker-compose.yml) file also shows the minimum setup that is needed for ARC to run.


## Monitoring

### Prometheus

Prometheus can collect ARC metrics. It improves observability in production and enables debugging during development and deployment. As Prometheus is a very standard tool for monitoring, any other complementary tool such as Grafana and others can be added for better data analysis.

Prometheus periodically poll the system data by querying specific urls.

ARC can expose a Prometheus endpoint that can be used to monitor the metamorph servers. Set the `prometheusEndpoint` setting in the settings file to activate prometheus. Normally you would want to set this to `/metrics`.

Enable monitoring consists of setting the **prometheusEndpoint** property in config.yaml file:

```yaml
prometheusEndpoint: /metrics # endpoint for prometheus metrics
```

### Profiler
Each service runs a http profiler server if it is configured in `config.yaml`. In order to access it, a connection can be created using the Go `pprof` [tool](https://pkg.go.dev/net/http/pprof). For example to investigate the memory usage
```bash
go tool pprof http://localhost:9999/debug/pprof/allocs
```
Then type `top` to see the functions which consume the most memory. Find more information [here](https://go.dev/blog/pprof).

### Tracing

In order to enable tracing for each service ther respective setting in the service has to be set in `config.yaml`

```yaml
  tracing:
    enabled: true # is tracing enabled
    dialAddr: http://localhost:4317 # address where traces are exported to
```

Currently the traces are exported only in [open telemtry protocol (OTLP)](https://opentelemetry.io/docs/specs/otel/protocol/) on the gRPC endpoint. This endpoint URL of the receiving tracing backend (e.g. [Jaeger](https://www.jaegertracing.io/), [Grafana Tempo](https://grafana.com/oss/tempo/), etc.) can be configured with the respective `tracing.dialAddr` setting.

## Building ARC

For building the ARC binary, there is a make target available. ARC can be built for Linux OS and amd64 architecture using

```
make build_release
```

Once this is done additionally a docker image can be built using

```
make build_docker
```

### Generate grpc code

GRPC code are generated from protobuf definitions. In order to generate the necessary tools need to be installed first by running
```
make install_gen
```
Additionally, [protoc](https://grpc.io/docs/protoc-installation/) needs to be installed.

Once that is done, GRPC code can be generated by running
```
make gen
```

### Generate REST API

The rest api is defined in a [yaml file](./api/arc.yml) following the OpenAPI 3.0.0 specification. Before the rest API can be generated install the necessary tools by running
```
make install_gen
```
Once that is done, the API code can be generated by running
```
make api
```

### Generate REST API documentation
Before the documentation can be generated [swagger-cli](https://apitools.dev/swagger-cli/) and [widdershins](https://github.com/Mermade/widdershins) need to be installed.

Once that is done the documentation can be created by running
```
make docs
```

# Acknowledgements
Special thanks to [rloadd](https://github.com/rloadd/) for his inputs to the documentation of ARC.
