package store

type Block struct {
	ID           int64  `db:"id"`
	Hash         []byte `db:"hash"`
	PreviousHash []byte `db:"prevhash"`
	MerkleRoot   []byte `db:"merkleroot"`
	Height       int64  `db:"height"`
	Orphaned     bool   `db:"orphaned"`
	Processed    bool   `db:"processed"`
}

type Transaction struct {
	ID         int64  `db:"id"`
	Hash       []byte `db:"hash"`
	Source     string `db:"source"`
	MerklePath string `db:"merkle_path"`
}

type BlockTransactionMap struct {
	BlockID       int64 `db:"blockid"`
	TransactionID int64 `db:"txid"`
	Pos           int64 `db:"pos"`
}
