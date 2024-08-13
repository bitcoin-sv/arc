-- Up Migration: Change the default value of 'competing_txs' to NULL
ALTER TABLE metamorph.transactions ALTER COLUMN competing_txs SET DEFAULT NULL;
