-- Up Migration: Add default constraint to 'status_history' column in 'metamorph.transactions' table
-- This migration sets the default value of the 'status_history' column to an empty JSON array ([]) for future records.

ALTER TABLE metamorph.transactions
    ALTER COLUMN status_history SET DEFAULT '[]'::JSONB;
