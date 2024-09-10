package config

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/libsv/go-p2p/wire"
	"github.com/spf13/viper"
)

var (
	ErrConfigFailedToDump   = errors.New("error occurred while dumping config")
	ErrConfigUnknownNetwork = errors.New("unknown bitcoin_network")
	ErrPortP2PNotSet        = errors.New("port_p2p not set for peer")
)

func DumpConfig(configFile string) error {
	err := viper.SafeWriteConfigAs(configFile)
	if err != nil {
		return errors.Join(ErrConfigFailedToDump, err)
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
		return 0, errors.Join(ErrConfigUnknownNetwork, fmt.Errorf("network: %s", networkStr))
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
		return "", errors.Join(ErrPortP2PNotSet, fmt.Errorf("peer host %s", p.Host))
	}

	return fmt.Sprintf("%s:%d", p.Host, p.Port.P2P), nil
}
