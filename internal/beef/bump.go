package beef

// BUMPs represents a slice of BUMPs - BSV Unified Merkle Paths
type BUMPs []*BUMP

// BUMP is a struct that represents a whole BUMP format
type BUMP struct {
	BlockHeight uint64
	Path        [][]BUMPLeaf
}

// BUMPLeaf represents each BUMP path element
type BUMPLeaf struct {
	Hash      string
	TxId      bool
	Duplicate bool
	Offset    uint64
}

// Flags which are used to determine the type of BUMPLeaf
const (
	dataFlag byte = iota
	duplicateFlag
	txIDFlag
)
