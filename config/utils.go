package config

import (
	"fmt"
	"net/url"

	"github.com/libsv/go-p2p/wire"
	"github.com/spf13/viper"
)

func DumpConfig(configFile string) error {
	err := viper.SafeWriteConfigAs(configFile)
	if err != nil {
		return fmt.Errorf("error while dumping config: %v", err.Error())
	}
	return nil
}

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

// GetZMQUrl gets the URL of the ZMQ port if available. If not available, nil is returned
func (p *PeerConfig) GetZMQUrl() (*url.URL, error) {
	if p.Port == nil || p.Port.ZMQ == 0 {
		return nil, nil
	}

	zmqURLString := fmt.Sprintf("zmq://%s:%d", p.Host, p.Port.ZMQ)

	return url.Parse(zmqURLString)
}

func (p *PeerConfig) GetP2PUrl() (string, error) {
	if p.Port == nil || p.Port.P2P == 0 {
		return "", fmt.Errorf("port_p2p not set for peer %s", p.Host)
	}

	return fmt.Sprintf("%s:%d", p.Host, p.Port.P2P), nil
}
