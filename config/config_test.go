package config

import (
	"testing"

	"github.com/libsv/go-bk/wif"
	"github.com/stretchr/testify/assert"
)

var exampleMinerKey = "KznvCNc6Yf4iztSThoMH6oHWzH9EgjfodKxmeuUGPq5DEX5maspS"
var exampleMinerID = "02798913bc057b344de675dac34faafe3dc2f312c758cd9068209f810877306d66"

func TestGetMinerID(t *testing.T) {
	t.Parallel()

	t.Run("no private key", func(t *testing.T) {
		minerIDConfig := MinerIDConfig{}
		minerID, err := minerIDConfig.GetMinerID()
		assert.ErrorIs(t, err, wif.ErrMalformedPrivateKey)
		assert.Equal(t, "", minerID)
	})

	t.Run("get Miner ID", func(t *testing.T) {
		minerIDConfig := MinerIDConfig{
			PrivateKey: exampleMinerKey,
		}
		minerID, err := minerIDConfig.GetMinerID()
		assert.NoError(t, err)
		assert.Equal(t, exampleMinerID, minerID)
	})
}
