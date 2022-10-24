package models

import (
	"testing"

	"github.com/mrz1836/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taal/mapi"
)

// TestPolicy_getPolicy will test the method getPolicy()
func TestPolicy_getPolicy(t *testing.T) {

	t.Run("GetPolicy empty", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		policy, err := GetPolicy(ctx, "test", WithClient(c))
		require.ErrorIs(t, err, datastore.ErrNoResults)
		assert.Nil(t, policy)
	})

	t.Run("getPolicy", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		_policy := &Policy{
			Model: *NewBaseModel(ModelNamePolicy, WithClient(c)),
			ID:    "test",
			Fees: Fees{{
				FeeType: "data",
				MiningFee: mapi.FeeAmount{
					Satoshis: 37,
					Bytes:    1000,
				},
				RelayFee: mapi.FeeAmount{
					Satoshis: 4,
					Bytes:    1000,
				},
			}},
			Policies: Policies{
				"skipscriptflags":               []string{"MINIMALDATA", "DERSIG", "NULLDUMMY", "DISCOURAGE_UPGRADABLE_NOPS", "CLEANSTACK"},
				"maxtxsizepolicy":               99999,
				"datacarriersize":               100000,
				"maxscriptsizepolicy":           100000,
				"maxscriptnumlengthpolicy":      100000,
				"maxstackmemoryusagepolicy":     10000000,
				"limitancestorcount":            1000,
				"limitcpfpgroupmemberscount":    10,
				"acceptnonstdoutputs":           true,
				"datacarrier":                   true,
				"dustrelayfee":                  150,
				"maxstdtxvalidationduration":    99,
				"maxnonstdtxvalidationduration": 100,
				"dustlimitfactor":               10,
			},
			ClientID: "",
		}
		err := _policy.Save(ctx)
		assert.NoError(t, err)

		var policy *Policy
		policy, err = GetPolicy(ctx, "test", WithClient(c))
		require.NoError(t, err)
		require.NotNil(t, policy)
		assert.Equal(t, mapi.FeeFeeType("data"), policy.Fees[0].FeeType)
		assert.Equal(t, uint64(37), policy.Fees[0].MiningFee.Satoshis)
	})
}

func TestPolicy_GetPolicyForClient(t *testing.T) {

	t.Run("GetPolicyForClient empty", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		policy, err := GetPolicyForClient(ctx, "test", WithClient(c))
		require.ErrorIs(t, err, datastore.ErrNoResults)
		assert.Nil(t, policy)
	})

	t.Run("GetDefaultPolicy", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		_policy := NewPolicy(WithClient(c))
		_policy.Fees = Fees{{
			FeeType: "standard",
			MiningFee: mapi.FeeAmount{
				Satoshis: 50,
				Bytes:    1000,
			},
			RelayFee: mapi.FeeAmount{
				Satoshis: 5,
				Bytes:    1000,
			},
		}, {
			FeeType: "data",
			MiningFee: mapi.FeeAmount{
				Satoshis: 37,
				Bytes:    1000,
			},
			RelayFee: mapi.FeeAmount{
				Satoshis: 4,
				Bytes:    1000,
			},
		}}
		err := _policy.Save(ctx)
		require.NoError(t, err)

		var policy *Policy
		policy, err = GetDefaultPolicy(ctx, WithClient(c))
		require.NoError(t, err)
		require.NotNil(t, policy)
		assert.Len(t, policy.Fees, 2)
	})

	t.Run("GetPolicyForClient", func(t *testing.T) {
		ctx, c, deferMe := CreateTestSQLiteClient(t, false, false)
		defer deferMe()

		_policy := NewPolicy(WithClient(c))
		_policy.Fees = Fees{{
			FeeType: "standard",
			MiningFee: mapi.FeeAmount{
				Satoshis: 50,
				Bytes:    1000,
			},
			RelayFee: mapi.FeeAmount{
				Satoshis: 5,
				Bytes:    1000,
			},
		}, {
			FeeType: "data",
			MiningFee: mapi.FeeAmount{
				Satoshis: 37,
				Bytes:    1000,
			},
			RelayFee: mapi.FeeAmount{
				Satoshis: 4,
				Bytes:    1000,
			},
		}}
		err := _policy.Save(ctx)
		require.NoError(t, err)

		var policy *Policy
		policy, err = GetDefaultPolicy(ctx, WithClient(c))
		require.NoError(t, err)
		require.NotNil(t, policy)
		assert.Len(t, policy.Fees, 2)
	})
}
