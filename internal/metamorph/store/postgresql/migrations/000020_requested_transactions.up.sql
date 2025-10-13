

ALTER TABLE global.Transactions ADD COLUMN requested_at TIMESTAMPTZ NULL;
ALTER TABLE global.Transactions ADD COLUMN confirmed_at TIMESTAMPTZ NULL;
