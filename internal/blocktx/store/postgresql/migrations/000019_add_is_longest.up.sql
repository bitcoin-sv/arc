-- field `is_longest` is an implementation detail that will help to
-- make sure that there is only one longest chain at any given height
-- and is also used as a helper when querying for longest chain
ALTER TABLE blocktx.blocks
ADD COLUMN is_longest BOOLEAN NOT NULL DEFAULT TRUE;

-- This will make is faster to search for blocks WHERE is_longest = true
CREATE INDEX ix_block_is_longest ON blocktx.blocks(is_longest);

-- This will make sure that there can only be ONE block at any
-- given height that is considered part of the LONGEST chain.
CREATE UNIQUE INDEX pux_height_is_longest ON blocktx.blocks(height)
WHERE is_longest;
