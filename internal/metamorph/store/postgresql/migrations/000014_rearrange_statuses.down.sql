-- QUEUED
UPDATE global.Transactions SET status = 1 WHERE status = 10;
-- RECEIVED
UPDATE global.Transactions SET status = 2 WHERE status = 20;
-- STORED
UPDATE global.Transactions SET status = 3 WHERE status = 30;
-- ANNOUNCED_TO_NETWORK
UPDATE global.Transactions SET status = 4 WHERE status = 40;
-- REQUESTED_BY_NETWORK
UPDATE global.Transactions SET status = 5 WHERE status = 50;
-- SENT_TO_NETWORK
UPDATE global.Transactions SET status = 6 WHERE status = 60;
-- ACCEPTED_BY_NETWORK
UPDATE global.Transactions SET status = 7 WHERE status = 70;

-- SEEN_IN_ORPHAN_MEMPOOL
UPDATE global.Transactions SET status = 10 WHERE status = 80;
-- SEEN_ON_NETWORK
UPDATE global.Transactions SET status = 8 WHERE status = 90;
-- REJECTED
UPDATE global.Transactions SET status = 109 WHERE status = 110;
-- MINED
UPDATE global.Transactions SET status = 9 WHERE status = 120;
