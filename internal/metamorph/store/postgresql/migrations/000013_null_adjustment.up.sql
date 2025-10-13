-- make full_status_updates not null
UPDATE metamorph.transactions
SET full_status_updates = false
WHERE full_status_updates IS NULL;

ALTER TABLE metamorph.transactions ALTER COLUMN full_status_updates SET NOT NULL;

-- make locked_by not null
UPDATE metamorph.transactions
SET locked_by = 'NONE'
WHERE locked_by IS NULL;

ALTER TABLE metamorph.transactions ALTER COLUMN locked_by SET NOT NULL;

-- make last_submitted_at not null
ALTER TABLE metamorph.transactions ALTER COLUMN last_submitted_at SET NOT NULL;

-- make stored_at not null
ALTER TABLE metamorph.transactions ALTER COLUMN stored_at SET NOT NULL;
