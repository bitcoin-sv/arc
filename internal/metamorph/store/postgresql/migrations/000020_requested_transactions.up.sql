

ALTER TABLE metamorph.transactions ADD COLUMN requested_at TIMESTAMPTZ NULL;
ALTER TABLE metamorph.transactions ADD COLUMN confirmed_at TIMESTAMPTZ NULL;
