-- Down Migration: Remove default constraint from 'status_history' column in 'metamorph.transactions' table
-- This migration removes the default value constraint from the 'status_history' column, so new records will require an explicit value or default to NULL.

ALTER TABLE metamorph.transactions
    ALTER COLUMN status_history DROP DEFAULT;
