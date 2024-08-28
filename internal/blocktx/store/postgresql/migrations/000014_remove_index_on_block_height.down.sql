CREATE UNIQUE INDEX pux_blocks_height ON blocks(height)
WHERE
    orphanedyn = FALSE;
