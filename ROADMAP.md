# ROADMAP

## Idempotent transactions

The capability of ARC to respond with the full block information including block hash, block height and Merkle path in case that a transaction is submitted to it which has been mined previously, but which was not submitted to that instance of ARC.

## Double spending detection

Introduction of a new status indicating that a transaction is in an DOUBLE_SPENT_ATTEMPTED state. All competing transactions progress in that status until they either get REJECTED or MINED. Extension of webhooks to notify parties if the transaction state changes due to double spend attempts.

## Multiple different callbacks per transaction

Submitting a transaction multiple times with different callback URL and/or callback token will at this pair as subscription to status updates. Callbacks will henceforth be sent to each callback url with specified callback token.

## Update of transactions in case of block reorgs

ARC updates the statuses of transactions in case of block reorgs. Transactions which are not in the block of the longest chain will be updated to `UNKNOWN` status and re-broadcasted. Transactions which are included in the block of the longest chain are updated to `MINED` status.
