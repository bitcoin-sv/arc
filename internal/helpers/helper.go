package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/libsv/go-p2p/wire"
	"github.com/lmittmann/tint"
	"github.com/spf13/viper"
)

func GetSlogLevel() (slog.Level, error) {

	logLevelString := viper.GetString("logLevel")

	switch logLevelString {
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	}

	return slog.LevelInfo, fmt.Errorf("invalid log level: %s", logLevelString)
}

func NewLogger() (*slog.Logger, error) {

	logLevel, err := GetSlogLevel()
	if err != nil {
		return nil, err
	}

	logFormat := viper.GetString("logFormat")
	switch logFormat {
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})), nil
	case "text":
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})), nil
	case "tint":
		return slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: logLevel})), nil
	}

	return nil, fmt.Errorf("invalid log format: %s", logFormat)
}

func GetString(settingName string) (string, error) {
	setting := viper.GetString(settingName)
	if setting == "" {
		return "", fmt.Errorf("setting %s not found", settingName)
	}

	return setting, nil
}

func GetInt(settingName string) (int, error) {
	setting := viper.GetInt(settingName)
	if setting == 0 {
		return 0, fmt.Errorf("setting %s not found", settingName)
	}

	return setting, nil
}

func GetDuration(settingName string) (time.Duration, error) {
	setting := viper.GetDuration(settingName)
	if setting == 0 {
		return 0, fmt.Errorf("setting %s not found", settingName)
	}

	return setting, nil
}

func GetInt64(settingName string) (int64, error) {
	setting := viper.GetInt64(settingName)
	if setting == 0 {
		return 0, fmt.Errorf("setting %s not found", settingName)
	}

	return setting, nil
}

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

func GetNetwork() (wire.BitcoinNet, error) {
	networkStr, err := GetString("network")
	if err != nil {
		return 0, err
	}

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
