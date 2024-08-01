# 400
ErrStatusBadRequest: The request seems to be malformed and cannot be processed

# 404
ErrStatusNotFound: The transaction you're looking for was not found in the database.

# 409
ErrStatusGeneric: This error has yet to be formally classified. We don't know what went wrong.

# 460
ErrStatusTxFormat: Transaction is not in extended format, and we were unable to enrich the data. Check that the inputs actually exist.

# 461
ErrStatusUnlockingScripts: One or more of the unlocking scripts did not validate against the corresponding locking script.

# 462
ErrStatusInputs: Either the input satoshis sum is too high, or there are no inputs specified, or the input is a coinbase transaction which is not currently supported.

# 463
ErrStatusOutputs: Transaction is invalid because the outputs are non-existent or attempting to create a non-zero false return output, or satoshi values greater than max value.

# 464
ErrStatusMalformed: Either the transaction is too small (61 bytes min), there are too many sig ops, or there is a non-data push in the unlocking script.

# 465
ErrStatusFees: The fees are too low, sum satoshis out is not less than sum satoshis in.

# 466
ErrStatusConflict: Transaction is invalid because the network has already seen a tx which spends the same utxo.

# 467
ErrStatusMinedAncestorsNotFound: invalid BEEF, cannot fined mined ancestor of the transaction.

# 468
ErrStatusCalculatingMerkleRoots: invalid BEEF, cannot calculate Merke Roots.

# 471
ErrStatusFrozenPolicy: Input Frozen (blacklist manager policy blacklisted). The transaction is attempting to spend frozen digital assets.

# 472
ErrStatusFrozenConsensus: Input Frozen (blacklist manager consensus blacklisted) The transaction is attempting to spend frozen digital assets.

# 473
ErrStatusCumulativeFees: Cumulative fees of all transactions in chain is too low.
