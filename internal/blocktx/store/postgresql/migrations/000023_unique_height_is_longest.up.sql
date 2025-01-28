-- This will make sure that there can only be ONE block at any
-- given height that is considered part of the LONGEST chain.
CREATE UNIQUE INDEX pux_height_is_longest ON blocktx.blocks(height)
WHERE is_longest;
