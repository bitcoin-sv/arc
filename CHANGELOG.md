# CHANGELOG

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Table of Contents
- [Unreleased](#unreleased)
- [1.5.0](#150---2025-09-22)
- [1.4.2](#142---2025-09-18)
- [1.4.1](#141---2025-09-10)
- [1.4.0](#140---2025-09-02)
- [1.3.54](#1354---2025-07-09)
- [1.3.20](#1320---2025-02-06)
- [1.3.13](#1313---2024-12-04)
- [1.3.12](#1312---2024-12-05)
- [1.3.2](#132---2024-10-30)
- [1.3.0](#130---2024-08-21)
- [1.2.0](#120---2024-08-13)
- [1.1.91](#1191---2024-06-26)
- [1.1.87](#1187---2024-06-10)
- [1.1.53](#1153---2024-04-11)
- [1.1.32](#1132---2024-02-21)
- [1.1.19](#1119---2024-02-05)
- [1.1.16](#1116---2024-01-23)
- [1.0.62](#1062---2023-11-23)
- [1.0.60](#1060---2023-11-21)
- [1.0.0 - YYYY-MM-DD](#100---yyyy-mm-dd)

## [Unreleased]

## [1.5.0] - 2025-09-22

### Changed
- Major refactoring of ARC configuration
  - ARC cli `-config` flag requires the exact path to the configuration file instead of the configuration path
  - Configuration can be combined from multiple configuration files
  - Settings which are valid used in multiple services are moved under a configuration common object

## [1.4.2] - 2025-09-18

### Changed
- New callbacker config setting `maxRetries`. It allows configuring the maximum number of times a callback is attempted to be sent.

## [1.4.1] - 2025-09-10

### Changed
- Callbacker loads a batch of unsent callbacks and sends them in chronological order per transaction.

## [1.4.0] - 2025-09-02

### Changed
- Callbacker stores all callbacks on the database. Any callbacker instance loads a batch of unsent callbacks and sends them in chronological order per URL. Once a callback was sent successfully, it is updated with a timestamp.

## [1.3.54] - 2025-07-09

### Added
- Transactions which have status SEEN_ON_NETWORK are marked as REJECTED by ARC if these transactions cannot be found in any connected peer's mempool and if a configured number of blocks have passed since the transaction was seen on the network. This should ensure that transactions are not stuck in SEEN_ON_NETWORK status forever. As a consequence, transactions submitted which have been mined earlier than the oldest block data available in `blocktx` will be marked as REJECTED as well.

## [1.3.20] - 2025-02-06

### Changed
- Callbacker sends the http messages in chronological order. If a callback fails Callbacker will resend the same callback until the callback is sent successfully, or it expires before it attempts to send the next callback.

## [1.3.13] - 2024-12-04

### Added
- [Reorg Support](https://bitcoin-sv.github.io/arc/#/?id=chain-reorg) - adapting ARC to handle chain reorganisations. Whenever reorg happens, ARC will update the block info for each transaction affected and rebroadcast those that are not seen in the newest longest chain.

## [1.3.12] - 2024-12-05

### Changed
- The Callbacker service handles unsuccessful callback attempts by delaying individual callbacks and retrying them again instead of putting an unsuccessful receiver in quarantine

### Added

## [1.3.2] - 2024-10-30

### Changed
- Callbacks are sent one by one to the same URL. In the previous implementation, each callback request created a new goroutine to send the callback, which could result in a potential DDoS of the callback receiver. The new approach sends callbacks to the same receiver in a serial manner. Note that URLs are not locked by the `callbacker` instance, so serial sends occur only within a single instance. In other words, the level of parallelism is determined by the number of `callbacker` instances.

- The Callbacker service handles unsuccessful callback attempts by placing problematic receivers in quarantine, temporarily pausing callback delivery to them.

### Added
- Callbacks are batched for transaction submitted with `X-CallbackBatch: true` header.

## [1.3.0] - 2024-08-21

### Changed
- The functionality for callbacks has been moved from the `metamorph` microservice to the new `callbacker` microservice.

## [1.2.0] - 2024-08-13

### Added
- [Double Spend Detection](https://bitcoin-sv.github.io/arc/#/?id=double-spending) is a feature that introduces `DOUBLE_SPEND_ATTEMPTED` status to transactions that attempt double spend together with `CompetingTxs` field in the API responses and callbacks.
- [Cumulative Fees Validation](https://bitcoin-sv.github.io/arc/#/?id=cumulative-fees-validation) is a feature that checks if a transaction has a sufficient fee not only for itself but also for all unmined ancestors that do not have sufficient fees.
- [Multiple callbacks to single transaction](https://bitcoin-sv.github.io/arc/#/?id=callbacks) Is a feature that adds support for attaching multiple callbacks to a single transaction when submitting an existing transaction with a different data.

## [1.1.91] - 2024-06-26

### Changed
- Idempotent transactions - when submitting an already mined but unregistered transaction, ARC will respond with either `ANNOUNCED_TO_NETWORK` or `SEEN_ON_NETWORK`. Additionally, ARC will send a callback with the `MINED` status, including block information and the Merkle Path.

## [1.1.87] - 2024-06-10

### Added
- [BEEF format](https://bsv.brc.dev/transactions/0062) is now supported. Simple Payment Verification (SPV) is performed on transactions coming in BEEF format, with a limited Merkle Roots verification against blocktx.

## [1.1.53] - 2024-04-11

### Deprecated

- Background-worker is removed. Deletion of data can be done by calling the respective rpc functions for [blocktx](./pkg/blocktx/blocktx_api/blocktx_api.proto) and [metamorph](./pkg/metamorph/metamorph_api/metamorph_api.proto) e.g. using a tool like [gRPCurl](https://github.com/fullstorydev/grpcurl).

## [1.1.32] - 2024-02-21

### Deprecated

- All database implementations in both `metamorph` and `blocktx` except Postgres have been removed in order to focus on one well tested implementation. Other implementations may be made available again at a later point in time. The commit where the db implementations were removed can be found here https://github.com/bitcoin-sv/arc/commit/053f7147ef41be2f5904c7d76ed51e91b7129780.

## [1.1.19] - 2024-02-05

### Deprecated

- Request header `X-MerkleProof` is deprecated and does no longer need to be provided. The Merkle proof is included in the MINED callback by default.

## [1.1.16] - 2024-01-23

### Added

- Callbacks for status `SEEN_ON_NETWORK` and `SEEN_IN_ORPHAN_MEMPOOL` if new request header `X-FullStatusUpdates` is given with value `true`.

## [1.0.62] - 2023-11-23

### Added

- Transaction status `SEEN_IN_ORPHAN_MEMPOOL`. The transaction has been sent to at least 1 Bitcoin node but parent transaction was not found. This status means that inputs are currently missing, but the transaction is not yet rejected.

### Changed

- A transaction for which a ZMQ message `missing inputs` of topic `invalidtx` is received, that transaction gets status `SEEN_IN_ORPHAN_MEMPOOL`.

## [1.0.60] - 2023-11-21

### Changed
- BREAKING CHANGE: The Merkle Path Binary format previously calculated and stored in BlockTx has been updated to a new standard encoding format referred to as BUMP and detailed here: [BRC-74](https://brc.dev/74). This means that the BlockTx database ought to be dumped prior to updating to using this version, since the structs are incompatible.

### Deprecated
- This has the effect of deprecating the previously used Merkle Path Binary format detailed here: [BRC-71](https://brc.dev/71) which is not used anywhere else in the ecosystem to our knowledge.

---

## [1.0.0] - YYYY-MM-DD

### Added
- Initial release

---

### Template for New Releases:

Replace `X.X.X` with the new version number and `YYYY-MM-DD` with the release date:

```
## [X.X.X] - YYYY-MM-DD

### Added
-

### Changed
-

### Deprecated
-

### Removed
-

### Fixed
-

### Security
-
```

Use this template as the starting point for each new version. Always update the "Unreleased" section with changes as they're implemented, and then move them under the new version header when that version is released.
