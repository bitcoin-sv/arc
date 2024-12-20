-- This will make is faster to search for transactions WHERE is_registered = true
CREATE INDEX ix_transaction_is_registered_hash ON blocktx.transactions(is_registered, hash);
