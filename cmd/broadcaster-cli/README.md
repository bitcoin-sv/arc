# Broadcaster-cli

The `broadcaster-cli` provides a set of functions for running performance tests on ARC. Additionally, `broadcaster-cli` allows the management of key sets and UTXOs.

## Table of contents

- [Broadcaster-cli](#broadcaster-cli)
  - [Table of contents](#table-of-contents)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [How to use broadcaster-cli to send batches of transactions to ARC](#how-to-use-broadcaster-cli-to-send-batches-of-transactions-to-arc)

## Installation

The broadcaster-cli can be installed using the following command.
```
go install github.com/bitcoin-sv/arc/cmd/broadcaster-cli@latest
```

If the ARC repository is checked out it can also be installed from that local repository like this
```
go install ./cmd/broadcaster-cli/
```

## Configuration

`broadcaster-cli` uses flags for adding context needed to run it. The flags and commands available can be shown by running `broadcaster-cli` with the flag `--help`.

As there can be a lot of flags you can also define them in a configuration file. The file [broadcaster-cli-example.yaml](./broadcaster-cli-example.yaml) is an example of the configuration. The format of the file can be the following: JSON, TOML, YAML, HCL, INI, envfile or Java properties formats.

A specific config file can be selected using the `--config` flag. Example:
```
broadcaster-cli keyset address -- --config ./cmd/broadcaster-cli/broadcaster-cli-example.yaml
```
Note that the config has to be added as a subcommand with a double dash `--` as shown above. The path to the config file has to be separated by a space.

If no config file is given using the `--config` flag, `broadcaster-cli` will search for `broadcaster-cli.yaml` in `.` and `./cmd/broadcaster-cli/` folders.

If a config file was found, then these values will be used as flags (if available to the command). You can still provide the flags, in which case the value provided in the flag will override the value provided in `broadcaster-cli.yaml`.

Note that a configuration file needs to be given at least for the private keys (see [broadcaster-cli-example.yaml](./broadcaster-cli-example.yaml)) as they cannot be passed as flags.

## How to use broadcaster-cli to send batches of transactions to ARC

These instructions will provide the steps needed in order to use `broadcaster-cli` to send transactions to ARC.

1. Create a new key set by running `broadcaster-cli keyset new`
    1. The key set displayed has to be added to the configuration file under `privateKeys`
2. Add funds to the funding addresses
    1. Show the funding addresses by running `broadcaster-cli keyset address`
       1. In case of `testnet` (using the `--testnet` flag) funds can be added using the WoC faucet. For that you can use the command `broadcaster-cli keyset topup --testnet`
       2. In case of `mainnet` funds could be added to one of the addresses.
   2. The funds can be spread to the other keys using the `utxos split` command.
      1. The following command will split funds of a given UTXO from `key-01` to keys: `key-01`, `key-02`, `key-03`, `key-04`
          ```
           broadcaster-cli utxos split --txid=cf111f19bcfb6baab7fc200f0f8fb669dd6c66fd9de212becb0950c92a0b6c40 --satoshis=21953 --vout=0 --from=key-01 --keys=key-01,key-02,key-03,key-04
          ```
      2. The same command can be used to move all funds from one UTXO from one to another key. The following example shows how to send all funds of the given UTXO from `key-01` to `key-02`
          ```
           broadcaster-cli utxos split --txid=cf111f19bcfb6baab7fc200f0f8fb669dd6c66fd9de212becb0950c92a0b6c40 --satoshis=21953 --vout=0 --from=key-01 --keys=key-02
          ```
      3. In order to just print the transaction without submitting it, the `--dryrun` flag can be added
   3. You can view the balance of the key set using the command `broadcaster-cli keyset balance`
3. Create UTXO set
    1. There must be a certain UTXO set available so that `broadcaster-cli` can broadcast a reasonable number of transactions in batches
    2. First look at the existing UTXO set using `broadcaster-cli keyset utxos`
    3. In order to create more outputs use the following command `broadcaster-cli utxos create --outputs=<number of outputs> --satoshis=<number of satoshis per output>`
    4. This command will send transactions creating the requested outputs to ARC. There are more flags needed for this command. Please see `go run cmd/broadcaster-cli/main.go utxos -h` for more details
    5. See the new distribution of UTXOs using `broadcaster-cli keyset utxos`
4. Broadcast transactions to ARC
    1. Now `broadcaster-cli` can be used to broadcast transactions to ARC at a given rate using this command `broadcaster-cli utxos broadcast --rate=<txs per second> --batchsize=<nr ot txs per batch>`
    2. The limit flag `--limit=<nr of transactions at which broadcasting stops>` is optional. If not given `broadcaster-cli` will only stop when `broadcaster-cli` is aborted manually e.g. using `CTRL+C`
    3. In order to broadcast a large number of transactions in parallel, multiple key sets can be given.
        1. Each concurrently running broadcasting process will broadcast at the given rate
        2. For example: If a rate of `--rate=100` is given with 3 key files `--keys=key-1,key-2,key-3`, then the final rate will be 300 transactions per second.
5. Consolidate outputs
    1. If not enough outputs are available for another test run it is best to consolidate the outputs so that there remains only 1 output using `broadcaster-cli utxos consolidate`
    2. After this step you can continue with step 4
        1. Before continuing with step 4 it is advisable to wait until all consolidation transactions were mined
        2. The command `broadcaster-cli keyset balance` shows the amount of satoshis in the balance that have been confirmed and the amount which has not yet been confirmed
