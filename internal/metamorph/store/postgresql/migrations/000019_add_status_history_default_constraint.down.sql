-- Down Migration: Remove default constraint from 'status_history' column in 'global.Transactions' table
-- This migration removes the default value constraint from the 'status_history' column, so new records will require an explicit value or default to NULL.

ALTER TABLE global.Transactions
    ALTER COLUMN status_history DROP DEFAULT;
