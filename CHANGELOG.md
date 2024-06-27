# CHANGELOG

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Table of Contents
- [Unreleased](#unreleased)
- [1.1.91](#1191---2024-06-26)
- [1.1.87](#1187---2024-06-10)
- [1.1.53](#1152---2024-04-11)
- [1.1.32](#1132---2024-02-21)
- [1.1.19](#1119---2024-02-05)
- [1.1.16](#1116---2024-01-23)
- [1.0.62](#1062---2023-11-23)
- [1.0.60](#1060---2023-11-21)
- [1.0.0 - YYYY-MM-DD](#100---yyyy-mm-dd)

## [Unreleased]

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
