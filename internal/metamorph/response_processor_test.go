package metamorph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bsv-blockchain/go-bt/v2/chainhash"
	"github.com/stretchr/testify/require"
)

func TestStatusResponse(t *testing.T) {
	testCases := []struct {
		name   string
		hash   *chainhash.Hash
		status metamorph_api.Status
		err    error

		expectedStatusAndError StatusAndError
	}{
		{
			name:   "announced_to_network",
			hash:   testdata.TX1HashB,
			status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			err:    nil,

			expectedStatusAndError: StatusAndError{
				Hash:   testdata.TX1HashB,
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
				Err:    nil,
			},
		},
		{
			name:   "announced_to_network - error",
			hash:   testdata.TX2HashB,
			status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			err:    errors.New("some error"),

			expectedStatusAndError: StatusAndError{
				Hash:   testdata.TX2HashB,
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
				Err:    errors.New("some error"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			statusChannel := make(chan StatusAndError, len(testCases))

			sut := NewStatusResponse(context.Background(), tc.hash, statusChannel)

			// when
			sut.UpdateStatus(StatusAndError{
				Status: tc.status,
				Err:    tc.err,
			})

			// then
			select {
			case retStatus := <-statusChannel:
				require.Equal(t, tc.expectedStatusAndError, retStatus)
			case <-time.After(time.Second):
				t.Fatal("test timout")
			}
		})
	}
}

func TestResponseProcessor(t *testing.T) {
	t.Run("test timeout", func(t *testing.T) {
		// given
		timeout := 100 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		sut := NewResponseProcessor()

		dummyStatus := NewStatusResponse(ctx, testdata.TX1HashB, nil)

		// when
		sut.Add(dummyStatus)

		time.Sleep(2 * timeout)

		// then
		require.Empty(t, sut.getMap())
	})

	t.Run("add and update status", func(t *testing.T) {
		// given
		sut := NewResponseProcessor()

		tx1Ch := make(chan StatusAndError, 1)
		tx2Ch := make(chan StatusAndError, 1)

		dummyStatus := NewStatusResponse(context.Background(), testdata.TX1HashB, tx1Ch)
		dummyStatus2 := NewStatusResponse(context.Background(), testdata.TX2HashB, tx2Ch)
		dummyStatus3 := NewStatusResponse(context.Background(), testdata.TX3HashB, nil)

		// when
		sut.Add(dummyStatus)
		sut.Add(dummyStatus2)
		sut.Add(dummyStatus3)

		require.Equal(t, 3, sut.getMapLen())

		sut.UpdateStatus(testdata.TX1HashB, StatusAndError{
			Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		})
		sut.UpdateStatus(testdata.TX2HashB, StatusAndError{
			Status: metamorph_api.Status_RECEIVED,
			Err:    errors.New("error for tx2"),
		})

		// then
		select {
		case res := <-tx1Ch:
			require.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, res.Status)
			require.Nil(t, res.Err)
		case res := <-tx2Ch:
			require.Equal(t, metamorph_api.Status_RECEIVED, res.Status)
			require.Equal(t, errors.New("error for tx2"), res.Err)
		case <-time.After(time.Second):
			t.Fatal("test timeout")
		}
	})
}
