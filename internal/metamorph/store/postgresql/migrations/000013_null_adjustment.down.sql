ALTER TABLE metamorph.transactions ALTER COLUMN stored_at DROP NOT NULL;

ALTER TABLE metamorph.transactions ALTER COLUMN last_submitted_at DROP NOT NULL;

ALTER TABLE metamorph.transactions ALTER COLUMN locked_by DROP NOT NULL;

ALTER TABLE metamorph.transactions ALTER COLUMN full_status_updates DROP NOT NULL;
