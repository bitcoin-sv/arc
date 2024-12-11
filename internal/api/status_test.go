package api

import (
	"github.com/bitcoin-sv/arc/pkg/api"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewErrorFields(t *testing.T) {
	tt := []struct {
		name   string
		status api.StatusCode

		expectedStatus api.StatusCode
	}{
		{
			name:   "ErrStatusBadRequest",
			status: api.ErrStatusBadRequest,

			expectedStatus: api.ErrStatusBadRequest,
		},
		{
			name:   "ErrStatusNotFound",
			status: api.ErrStatusNotFound,

			expectedStatus: api.ErrStatusNotFound,
		},
		{
			name:   "ErrStatusGeneric",
			status: api.ErrStatusGeneric,

			expectedStatus: api.ErrStatusGeneric,
		},
		{
			name:   "ErrStatusTxFormat",
			status: api.ErrStatusTxFormat,

			expectedStatus: api.ErrStatusTxFormat,
		},
		{
			name:   "ErrStatusUnlockingScripts",
			status: api.ErrStatusUnlockingScripts,

			expectedStatus: api.ErrStatusUnlockingScripts,
		},
		{
			name:   "ErrStatusInputs",
			status: api.ErrStatusInputs,

			expectedStatus: api.ErrStatusInputs,
		},
		{
			name:   "ErrStatusOutputs",
			status: api.ErrStatusOutputs,

			expectedStatus: api.ErrStatusOutputs,
		},
		{
			name:   "ErrStatusMalformed",
			status: api.ErrStatusMalformed,

			expectedStatus: api.ErrStatusMalformed,
		},
		{
			name:   "ErrStatusFees",
			status: api.ErrStatusFees,

			expectedStatus: api.ErrStatusFees,
		},
		{
			name:   "ErrStatusConflict",
			status: api.ErrStatusConflict,

			expectedStatus: api.ErrStatusConflict,
		},
		{
			name:   "ErrStatusFrozenPolicy",
			status: api.ErrStatusFrozenPolicy,

			expectedStatus: api.ErrStatusFrozenPolicy,
		},
		{
			name:   "ErrStatusFrozenConsensus",
			status: api.ErrStatusFrozenConsensus,

			expectedStatus: api.ErrStatusFrozenConsensus,
		},
		{
			name:   "non existent status",
			status: 1000,

			expectedStatus: api.ErrStatusGeneric,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			errFields := api.NewErrorFields(tc.status, "some extra info")

			require.Equal(t, int(tc.expectedStatus), errFields.Status)
			require.Equal(t, "some extra info", *errFields.ExtraInfo)
		})
	}
}
