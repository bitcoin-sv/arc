CREATE INDEX idx_metamorph_transactions_locked_by_status ON metamorph.transactions(locked_by, status);
