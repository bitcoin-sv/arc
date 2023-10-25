package store

import "time"

type Block struct {
	ID           int        `db:"id"`
	Hash         string     `db:"hash"`
	PreviousHash string     `db:"prevhash"`
	MerkleRoot   string     `db:"merkleroot"`
	Height       int      `db:"height"`
	Orphaned     bool       `db:"orphanedyn"`
	Processed    bool       `db:"processed"`
	ProcessedAt  *time.Time `db:"processed_at"`
	InsertedAt   *time.Time `db:"inserted_at"`
	Size         *int64     `db:"size"`
	TxCount      *int64     `db:"tx_count"`
	MerklePath   *string    `db:"merkle_path"`
}

type Transaction struct {
	ID         int    `db:"id"`
	Hash       string `db:"hash"`
	Source     string `db:"source"`
	MerklePath string `db:"merkle_path"`
}

type BlockTransactionMap struct {
	BlockID       int   `db:"blockid"`
	TransactionID int   `db:"txid"`
	Pos           int64 `db:"pos"`
}
