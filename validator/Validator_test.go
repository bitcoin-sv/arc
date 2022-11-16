package validator

import (
	"bytes"
	"testing"
)

func TestMap(t *testing.T) {
	m := make(map[Outpoint]OutpointData)

	o := Outpoint{
		Txid: "a1199d9d914d94fce621e57986bdebf07467dc99ab61606804ae10fe398c96a7",
		Idx:  0,
	}

	od := OutpointData{
		ScriptPubKey: []byte{0x01, 0x02, 0x03},
		Satoshis:     123,
	}

	m[o] = od

	od2 := m[o]

	if !bytes.Equal(od.ScriptPubKey, od2.ScriptPubKey) {
		t.Errorf("Expected %x, got %x", od.ScriptPubKey, od2.ScriptPubKey)
	}

	if od.Satoshis != od2.Satoshis {
		t.Errorf("Expected %d, got %d", od.Satoshis, od2.Satoshis)
	}

}
