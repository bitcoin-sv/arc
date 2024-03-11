ALTER TABLE metamorph.transactions ADD COLUMN merkle_path TEXT DEFAULT '' :: TEXT;

DROP TABLE metamorph.blocks;
