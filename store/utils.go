package store

import (
	"encoding/hex"

	"github.com/ordishs/go-utils"

	"github.com/libsv/go-bt/v2"
)

func HashString(b []byte) string {
	return hex.EncodeToString(bt.ReverseBytes(utils.Sha256d(b)))
}

func SplitStringDelimiter(s string, delim string) []string {
	var result []string
	for _, v := range s {
		if string(v) == delim {
			result = append(result, "")
		} else {
			result[len(result)-1] += string(v)
		}
	}
	return result

}
