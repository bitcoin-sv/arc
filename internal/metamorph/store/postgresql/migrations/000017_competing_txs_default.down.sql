-- Down Migration: Revert the default value of 'competing_txs' to an empty string ''
ALTER TABLE metamorph.transactions ALTER COLUMN competing_txs SET DEFAULT '';
