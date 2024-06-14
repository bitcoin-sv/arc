package config

import (
	"fmt"
	"net/url"

	"github.com/libsv/go-p2p/wire"
)

func GetNetwork(networkStr string) (wire.BitcoinNet, error) {
	var network wire.BitcoinNet

	switch networkStr {
	case "mainnet":
		network = wire.MainNet
	case "testnet":
		network = wire.TestNet3
	case "regtest":
		network = wire.TestNet
	default:
		return 0, fmt.Errorf("unknown bitcoin_network: %s", networkStr)
	}

	return network, nil
}

func (p *PeerConfig) GetZMQUrl() (*url.URL, error) {
	if p.Port.ZMQ == 0 {
		return nil, fmt.Errorf("port_zmq not set for peer %s", p.Host)
	}

	zmqURLString := fmt.Sprintf("zmq://%s:%d", p.Host, p.Port.ZMQ)

	return url.Parse(zmqURLString)
}

func (p *PeerConfig) GetP2PUrl() (string, error) {
	if p.Port.P2P == 0 {
		return "", fmt.Errorf("port_p2p not set for peer %s", p.Host)
	}

	return fmt.Sprintf("%s:%d", p.Host, p.Port.P2P), nil
}
