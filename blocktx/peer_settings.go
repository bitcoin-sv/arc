package blocktx

import (
	"fmt"
	"net/url"

	"github.com/spf13/viper"
)

type Peer struct {
	Host string
	Port PeerPort `mapstructure:"port"`
}

type PeerPort struct {
	P2P int `mapstructure:"p2p"`
	ZMQ int `mapstructure:"zmq"`
}

func (p Peer) GetZMQUrl() (*url.URL, error) {
	if p.Port.ZMQ == 0 {
		return nil, fmt.Errorf("port_zmq not set for peer %s", p.Host)
	}

	zmqURLString := fmt.Sprintf("zmq://%s:%d", p.Host, p.Port.ZMQ)

	return url.Parse(zmqURLString)
}

func (p Peer) GetP2PUrl() (string, error) {
	if p.Port.P2P == 0 {
		return "", fmt.Errorf("port_p2p not set for peer %s", p.Host)
	}

	return fmt.Sprintf("%s:%d", p.Host, p.Port.P2P), nil
}

func GetPeerSettings() ([]Peer, error) {
	var peers []Peer
	err := viper.UnmarshalKey("peers", &peers)
	if err != nil {
		return []Peer{}, err
	}

	if len(peers) == 0 {
		return []Peer{}, fmt.Errorf("no peers set")
	}

	for i, peer := range peers {
		if peer.Host == "" {
			return []Peer{}, fmt.Errorf("host not set for peer %d", i+1)
		}
	}

	return peers, nil
}
