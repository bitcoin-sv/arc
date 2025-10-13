-- Up Migration: Change the default value of 'competing_txs' to NULL
ALTER TABLE global.Transactions ALTER COLUMN competing_txs SET DEFAULT NULL;
