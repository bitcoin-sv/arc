create table transactions (
                              id bigint NOT NULL,
                              hash bytea NOT NULL,
                              source text,
                              merkle_path text DEFAULT ''::text
);