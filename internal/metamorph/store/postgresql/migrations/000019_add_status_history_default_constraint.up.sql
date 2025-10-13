-- Up Migration: Add default constraint to 'status_history' column in 'global.Transactions' table
-- This migration sets the default value of the 'status_history' column to an empty JSON array ([]) for future records.

ALTER TABLE global.Transactions
    ALTER COLUMN status_history SET DEFAULT '[]'::JSONB;
