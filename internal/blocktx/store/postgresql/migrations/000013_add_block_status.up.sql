ALTER TABLE blocktx.blocks
ADD COLUMN status INTEGER NOT NULL DEFAULT 10, -- 10 is equal to status LONGEST
ADD COLUMN chainwork TEXT NOT NULL DEFAULT '0'; -- chainwork is of type *big.Int, stored as TEXT for simplicity
