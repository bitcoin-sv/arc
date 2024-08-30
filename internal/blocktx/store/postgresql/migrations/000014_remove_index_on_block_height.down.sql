CREATE UNIQUE INDEX blocktx.pux_blocks_height ON blocktx.blocks(height)
WHERE
    orphanedyn = FALSE;
