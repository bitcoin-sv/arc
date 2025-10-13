ALTER TABLE global.Transactions ADD COLUMN inserted_at TIMESTAMPTZ;
ALTER TABLE metamorph.blocks ADD COLUMN inserted_at TIMESTAMPTZ;
