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
			expectedNetwork: 0, // invlid network
			expectedError:   errors.New("unknown bitcoin_network: invalidnet"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			net, err := GetNetwork(tc.networkStr)

			assert.Equal(t, tc.expectedNetwork, net)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func Test_GetZMQUrl_GetP2PUrl(t *testing.T) {
	testCases := []struct {
		name             string
		peerConfig       *PeerConfig
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
			expectedP2PUrl:   "localhost:18332",
			expectedZMQUrl:   "",
			expectedP2PError: nil,
			expectedZMQError: errors.New("port_zmq not set for peer localhost"),
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
			expectedP2PError: errors.New("port_p2p not set for peer localhost"),
			expectedZMQError: nil,
		},
		{
			name: "no port configuration",
			peerConfig: &PeerConfig{
				Host: "localhost",
			},
			expectedP2PUrl:   "",
			expectedZMQUrl:   "",
			expectedP2PError: errors.New("port_p2p not set for peer localhost"),
			expectedZMQError: errors.New("port_zmq not set for peer localhost"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			zmqUrl, zmqErr := tc.peerConfig.GetZMQUrl()
			p2pUrl, p2pErr := tc.peerConfig.GetP2PUrl()

			assert.Equal(t, tc.expectedZMQError, zmqErr)
			assert.Equal(t, tc.expectedP2PError, p2pErr)
			assert.Equal(t, tc.expectedP2PUrl, p2pUrl)
			if tc.expectedZMQError == nil {
				assert.Equal(t, tc.expectedZMQUrl, zmqUrl.String())
			}
		})
	}
}
