CREATE INDEX idx_metamorph_transactions_locked_by_status ON global.Transactions(locked_by, status);
