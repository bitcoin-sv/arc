package models

import (
	"testing"

	"github.com/mrz1836/go-datastore"
	"github.com/stretchr/testify/assert"
)

// TestUtxo_getUtxo will test the method getUtxo()
func TestFee_getFee(t *testing.T) {

	t.Run("GetFee empty", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		fee, err := GetFee(ctx, "test", WithClient(c))
		assert.ErrorIs(t, err, datastore.ErrNoResults)
		assert.Nil(t, fee)
	})

	t.Run("getUtxo", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		_fee := &Fee{
			Model:   *NewBaseModel(ModelNameFee, WithClient(c)),
			ID:      "test",
			FeeType: "data",
			MiningFee: FeeAmount{
				Satoshis: 37,
				Bytes:    1000,
			},
			RelayFee: FeeAmount{
				Satoshis: 4,
				Bytes:    1000,
			},
			ClientID: "",
		}
		err := _fee.Save(ctx)
		assert.NoError(t, err)

		var fee *Fee
		fee, err = GetFee(ctx, "test", WithClient(c))
		assert.NoError(t, err)
		assert.NotNil(t, fee)
		assert.Equal(t, "data", fee.FeeType)
		assert.Equal(t, uint64(37), fee.MiningFee.Satoshis)
	})
}

func TestFee_getFeesForClient(t *testing.T) {

	t.Run("getFees empty", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		fee, err := GetFeesForClient(ctx, "test", WithClient(c))
		assert.ErrorIs(t, err, datastore.ErrNoResults)
		assert.Nil(t, fee)
	})

	t.Run("getFees", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		_fee := NewFee(WithClient(c))
		_fee.FeeType = "standard"
		_fee.MiningFee = FeeAmount{
			Satoshis: 50,
			Bytes:    1000,
		}
		_fee.RelayFee = FeeAmount{
			Satoshis: 5,
			Bytes:    1000,
		}
		err := _fee.Save(ctx)
		assert.NoError(t, err)

		_fee = NewFee(WithClient(c))
		_fee.FeeType = "data"
		_fee.MiningFee = FeeAmount{
			Satoshis: 37,
			Bytes:    1000,
		}
		_fee.RelayFee = FeeAmount{
			Satoshis: 4,
			Bytes:    1000,
		}
		err = _fee.Save(ctx)
		assert.NoError(t, err)

		var fees []*Fee
		fees, err = GetFeesForClient(ctx, "", WithClient(c))
		assert.NoError(t, err)
		assert.Len(t, fees, 2)
	})
}
