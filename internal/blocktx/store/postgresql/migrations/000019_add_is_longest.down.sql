ALTER TABLE blocktx.blocks
DROP INDEX pux_height_is_longest,
DROP INDEX ix_block_is_longest,
DROP COLUMN is_longest;
