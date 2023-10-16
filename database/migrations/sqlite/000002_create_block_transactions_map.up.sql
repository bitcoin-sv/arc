CREATE TABLE block_transactions_map (
                                        blockid INTEGER NOT NULL,
                                        txid INTEGER NOT NULL,
                                        pos INTEGER NOT NULL,
                                        PRIMARY KEY (blockid, txid)
);