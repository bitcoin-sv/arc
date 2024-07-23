-- QUEUED
UPDATE metamorph.transactions SET status = 10 WHERE status = 1;
-- RECEIVED
UPDATE metamorph.transactions SET status = 20 WHERE status = 2;
-- STORED
UPDATE metamorph.transactions SET status = 30 WHERE status = 3;
-- ANNOUNCED_TO_NETWORK
UPDATE metamorph.transactions SET status = 40 WHERE status = 4;
-- REQUESTED_BY_NETWORK
UPDATE metamorph.transactions SET status = 50 WHERE status = 5;
-- SENT_TO_NETWORK
UPDATE metamorph.transactions SET status = 60 WHERE status = 6;
-- ACCEPTED_BY_NETWORK
UPDATE metamorph.transactions SET status = 70 WHERE status = 7;

-- SEEN_IN_ORPHAN_MEMPOOL
UPDATE metamorph.transactions SET status = 80 WHERE status = 10;
-- SEEN_ON_NETWORK
UPDATE metamorph.transactions SET status = 90 WHERE status = 8;
-- REJECTED
UPDATE metamorph.transactions SET status = 110 WHERE status = 109;
-- MINED
UPDATE metamorph.transactions SET status = 120 WHERE status = 9;
