ALTER TABLE global.Transactions ADD COLUMN merkle_path TEXT DEFAULT '' :: TEXT;

DROP TABLE metamorph.blocks;
