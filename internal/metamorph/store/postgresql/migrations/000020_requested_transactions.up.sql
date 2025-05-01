

CREATE TABLE metamorph.requested_transactions (
  hash BYTEA PRIMARY KEY,
  requested_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
  confirmed_at TIMESTAMPTZ NULL
)
