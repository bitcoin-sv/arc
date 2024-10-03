package config

import (
	"errors"
	"testing"

	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/assert"
)

func Test_GetNetwork(t *testing.T) {
	testCases := []struct {
		name            string
		networkStr      string
		expectedNetwork wire.BitcoinNet
		expectedError   error
	}{
		{
			name:            "mainnet",
			networkStr:      "mainnet",
			expectedNetwork: wire.MainNet,
			expectedError:   nil,
		},
		{
			name:            "testnet",
			networkStr:      "testnet",
			expectedNetwork: wire.TestNet3,
			expectedError:   nil,
		},
		{
			name:            "regtest",
			networkStr:      "regtest",
			expectedNetwork: wire.TestNet,
			expectedError:   nil,
		},
		{
			name:            "invalid network",
			networkStr:      "invalidnet",
			expectedNetwork: 0, // invalid network
			expectedError:   ErrConfigUnknownNetwork,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			actualNetwork, err := GetNetwork(tc.networkStr)

			// then
			assert.Equal(t, tc.expectedNetwork, actualNetwork)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func Test_GetZMQUrl_GetP2PUrl(t *testing.T) {
	testCases := []struct {
		name       string
		peerConfig *PeerConfig

		expectedZmqIsNil bool
		expectedP2PUrl   string
		expectedZMQUrl   string
		expectedP2PError error
		expectedZMQError error
	}{
		{
			name: "valid config",
			peerConfig: &PeerConfig{
				Host: "localhost",
				Port: &PeerPortConfig{
					P2P: 18332,
					ZMQ: 28332,
				},
			},
			expectedP2PUrl:   "localhost:18332",
			expectedZMQUrl:   "zmq://localhost:28332",
			expectedP2PError: nil,
			expectedZMQError: nil,
		},
		{
			name: "zmq port missing",
			peerConfig: &PeerConfig{
				Host: "localhost",
				Port: &PeerPortConfig{
					P2P: 18332,
				},
			},

			expectedZmqIsNil: true,
			expectedP2PUrl:   "localhost:18332",
			expectedZMQUrl:   "",
			expectedP2PError: nil,
			expectedZMQError: nil,
		},
		{
			name: "p2p port missing",
			peerConfig: &PeerConfig{
				Host: "localhost",
				Port: &PeerPortConfig{
					ZMQ: 28332,
				},
			},
			expectedP2PUrl:   "",
			expectedZMQUrl:   "zmq://localhost:28332",
			expectedP2PError: ErrPortP2PNotSet,
			expectedZMQError: nil,
		},
		{
			name: "no port configuration",
			peerConfig: &PeerConfig{
				Host: "localhost",
			},
			expectedP2PUrl:   "",
			expectedZMQUrl:   "",
			expectedZmqIsNil: true,
			expectedP2PError: ErrPortP2PNotSet,
			expectedZMQError: errors.New("port_zmq not set for peer localhost"), // external error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			actualP2PURL, actualP2PErr := tc.peerConfig.GetP2PUrl()
			actualZmqURL, actualZmqErr := tc.peerConfig.GetZMQUrl()

			// then
			assert.ErrorIs(t, actualP2PErr, tc.expectedP2PError)
			assert.Equal(t, tc.expectedP2PUrl, actualP2PURL)

			if tc.expectedZmqIsNil {
				assert.Nil(t, actualZmqURL)
				return
			}

			assert.NotNil(t, actualZmqURL)
			assert.ErrorIs(t, actualZmqErr, tc.expectedZMQError)
			if tc.expectedZMQError == nil {
				assert.Equal(t, tc.expectedZMQUrl, actualZmqURL.String())
			}
		})
	}
}
