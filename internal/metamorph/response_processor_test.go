package metamorph

import (
	"errors"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
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
			hash:   testdata.TX1Hash,
			status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			err:    nil,

			expectedStatusAndError: StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
				Err:    nil,
			},
		},
		{
			name:   "announced_to_network - error",
			hash:   testdata.TX1Hash,
			status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			err:    errors.New("some error"),

			expectedStatusAndError: StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
				Err:    errors.New("some error"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			statusChannel := make(chan StatusAndError, len(testCases))

			statusResponse := NewStatusResponse(tc.hash, statusChannel)

			statusResponse.UpdateStatus(tc.status, tc.err)

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
		rp := NewResponseProcessor()

		dummyStatus := NewStatusResponse(testdata.TX1Hash, nil)

		timeout := 100 * time.Millisecond
		rp.Add(dummyStatus, timeout)

		time.Sleep(2 * timeout)

		require.Empty(t, rp.responseMap)
	})

	t.Run("add and update status", func(t *testing.T) {
		rp := NewResponseProcessor()

		dummyStatus := NewStatusResponse(testdata.TX1Hash, nil)
		dummyStatus2 := NewStatusResponse(testdata.TX2Hash, nil)
		dummyStatus3 := NewStatusResponse(testdata.TX3Hash, nil)

		rp.Add(dummyStatus, time.Second)
		rp.Add(dummyStatus2, time.Second)
		rp.Add(dummyStatus3, time.Second)

		require.Len(t, rp.responseMap, 3)
		require.Equal(t, metamorph_api.Status_RECEIVED, rp.responseMap[testdata.TX1Hash].Status)
		require.Nil(t, rp.responseMap[testdata.TX1Hash].Err)
		require.Equal(t, metamorph_api.Status_RECEIVED, rp.responseMap[testdata.TX2Hash].Status)
		require.Nil(t, rp.responseMap[testdata.TX2Hash].Err)
		require.Equal(t, metamorph_api.Status_RECEIVED, rp.responseMap[testdata.TX3Hash].Status)
		require.Nil(t, rp.responseMap[testdata.TX3Hash].Err)

		rp.UpdateStatus(testdata.TX1Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, nil)
		rp.UpdateStatus(testdata.TX2Hash, metamorph_api.Status_RECEIVED, errors.New("error for tx2"))

		require.Len(t, rp.responseMap, 3)
		require.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, rp.responseMap[testdata.TX1Hash].Status)
		require.Nil(t, rp.responseMap[testdata.TX1Hash].Err)
		require.Equal(t, metamorph_api.Status_RECEIVED, rp.responseMap[testdata.TX2Hash].Status)
		require.Equal(t, errors.New("error for tx2"), rp.responseMap[testdata.TX2Hash].Err)
		require.Equal(t, metamorph_api.Status_RECEIVED, rp.responseMap[testdata.TX3Hash].Status)
		require.Nil(t, rp.responseMap[testdata.TX3Hash].Err)
	})
}
