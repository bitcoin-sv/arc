# ROADMAP

## Update of transactions in case of block reorgs

ARC updates the statuses of transactions in case of block reorgs. Transactions which are not in the block of the longest chain will be updated to `UNKNOWN` status and re-broadcasted. Transactions which are included in the block of the longest chain are updated to `MINED` status.
