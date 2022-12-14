--liquibase formatted sql

--changeset ordidshs:C1 stripComments:false runOnChange:false splitStatements:false
--comment:Create blocks table.
CREATE TABLE blocks (
 id           BIGSERIAL PRIMARY KEY,
 hash         BYTEA NOT NULL
,header       BYTEA NOT NULL
,height       BIGINT NOT NULL
,processedyn  BOOLEAN NOT NULL DEFAULT FALSE
,orphanedyn   BOOLEAN NOT NULL DEFAULT FALSE
);

GRANT ALL ON TABLE blocks TO blocktx;

CREATE UNIQUE INDEX ON blocks (hash);

CREATE UNIQUE INDEX PARTIAL ON blocks(height) WHERE orphanedyn = FALSE;


--changeset ordishs:C2 stripComments:false runOnChange:false splitStatements:false
--comment:Create block_transactions table.
CREATE TABLE block_transactions (
 blockid      BIGINT NOT NULL REFERENCES blocks(id)
,txhash       BYTEA NOT NULL
,PRIMARY KEY (blockid, txhash)
);

GRANT ALL ON TABLE block_transactions TO blocktx;

