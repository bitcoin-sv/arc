package beef

const (
	versionBytesCount = 2
	BEEFMarkerPart1   = 0xBE
	BEEFMarkerPart2   = 0xEF
)

func CheckBeefFormat(txHex []byte) bool {
	// removes version bytes
	txHex = txHex[versionBytesCount:]

	if txHex[0] != BEEFMarkerPart1 || txHex[1] != BEEFMarkerPart2 {
		return false
	}

	return true
}
