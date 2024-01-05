package store

import "time"

type Block struct {
	Hash          string    `db:"hash"`
	ProcessedAt   time.Time `db:"processed_at"`
	InsertedAt    time.Time `db:"inserted_at"`
	InsertedAtNum int64     `db:"inserted_at_num"`
}

type Transaction struct {
	Hash          string    `db:"hash"`
	StoredAt      time.Time `db:"stored_at"`
	AnnouncedAt   time.Time `db:"announced_at"`
	MinedAt       time.Time `db:"mined_at"`
	Status        int       `db:"status"`
	BlockHeight   int64     `db:"block_height"`
	BlockHash     []byte    `db:"block_hash"`
	CallbackURL   string    `db:"callback_url"`
	CallbackToken string    `db:"callback_token"`
	MerkleProof   string    `db:"merkle_proof"`
	RejectReason  string    `db:"reject_reason"`
	RawTX         []byte    `db:"raw_tx"`
	LockedBy      string    `db:"locked_by"`
	InsertedAt    time.Time `db:"inserted_at"`
	InsertedAtNum int64     `db:"inserted_at_num"`
}
