package p2p

type PeerManagerI interface {
	AnnounceNewTransaction(txID []byte)
}
