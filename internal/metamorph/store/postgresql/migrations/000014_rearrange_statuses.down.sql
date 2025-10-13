-- QUEUED
UPDATE metamorph.transactions SET status = 1 WHERE status = 10;
-- RECEIVED
UPDATE metamorph.transactions SET status = 2 WHERE status = 20;
-- STORED
UPDATE metamorph.transactions SET status = 3 WHERE status = 30;
-- ANNOUNCED_TO_NETWORK
UPDATE metamorph.transactions SET status = 4 WHERE status = 40;
-- REQUESTED_BY_NETWORK
UPDATE metamorph.transactions SET status = 5 WHERE status = 50;
-- SENT_TO_NETWORK
UPDATE metamorph.transactions SET status = 6 WHERE status = 60;
-- ACCEPTED_BY_NETWORK
UPDATE metamorph.transactions SET status = 7 WHERE status = 70;

-- SEEN_IN_ORPHAN_MEMPOOL
UPDATE metamorph.transactions SET status = 10 WHERE status = 80;
-- SEEN_ON_NETWORK
UPDATE metamorph.transactions SET status = 8 WHERE status = 90;
-- REJECTED
UPDATE metamorph.transactions SET status = 109 WHERE status = 110;
-- MINED
UPDATE metamorph.transactions SET status = 9 WHERE status = 120;
