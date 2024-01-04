package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewErrorFields(t *testing.T) {
	tt := []struct {
		name   string
		status StatusCode

		expectedStatus StatusCode
	}{
		{
			name:   "ErrStatusBadRequest",
			status: ErrStatusBadRequest,

			expectedStatus: ErrStatusBadRequest,
		},
		{
			name:   "ErrStatusNotFound",
			status: ErrStatusNotFound,

			expectedStatus: ErrStatusNotFound,
		},
		{
			name:   "ErrStatusGeneric",
			status: ErrStatusGeneric,

			expectedStatus: ErrStatusGeneric,
		},
		{
			name:   "ErrStatusTxFormat",
			status: ErrStatusTxFormat,

			expectedStatus: ErrStatusTxFormat,
		},
		{
			name:   "ErrStatusUnlockingScripts",
			status: ErrStatusUnlockingScripts,

			expectedStatus: ErrStatusUnlockingScripts,
		},
		{
			name:   "ErrStatusInputs",
			status: ErrStatusInputs,

			expectedStatus: ErrStatusInputs,
		},
		{
			name:   "ErrStatusOutputs",
			status: ErrStatusOutputs,

			expectedStatus: ErrStatusOutputs,
		},
		{
			name:   "ErrStatusMalformed",
			status: ErrStatusMalformed,

			expectedStatus: ErrStatusMalformed,
		},
		{
			name:   "ErrStatusFees",
			status: ErrStatusFees,

			expectedStatus: ErrStatusFees,
		},
		{
			name:   "ErrStatusConflict",
			status: ErrStatusConflict,

			expectedStatus: ErrStatusConflict,
		},
		{
			name:   "ErrStatusFrozenPolicy",
			status: ErrStatusFrozenPolicy,

			expectedStatus: ErrStatusFrozenPolicy,
		},
		{
			name:   "ErrStatusFrozenConsensus",
			status: ErrStatusFrozenConsensus,

			expectedStatus: ErrStatusFrozenConsensus,
		},
		{
			name:   "non existent status",
			status: 1000,

			expectedStatus: ErrStatusGeneric,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			errFields := NewErrorFields(tc.status, "some extra info")

			require.Equal(t, int(tc.expectedStatus), errFields.Status)
			require.Equal(t, "some extra info", *errFields.ExtraInfo)
		})
	}
}
