-- make full_status_updates not null
UPDATE global.Transactions
SET full_status_updates = false
WHERE full_status_updates IS NULL;

ALTER TABLE global.Transactions ALTER COLUMN full_status_updates SET NOT NULL;

-- make locked_by not null
UPDATE global.Transactions
SET locked_by = 'NONE'
WHERE locked_by IS NULL;

ALTER TABLE global.Transactions ALTER COLUMN locked_by SET NOT NULL;

-- make last_submitted_at not null
ALTER TABLE global.Transactions ALTER COLUMN last_submitted_at SET NOT NULL;

-- make stored_at not null
ALTER TABLE global.Transactions ALTER COLUMN stored_at SET NOT NULL;
