package config

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type ArcConfig struct {
	// root config
	PeerRpc *PeerRpcConfig `json:"peerRpc" mapstructure:"peerRpc"`
	// peers
	// metamorph
	// blocktx
	// api
	// k8sWatcher
}

type PeerRpcConfig struct {
	Password string `json:"password" mapstructure:"password"`
	User     string `json:"user" mapstructure:"user"`
	Host     string `json:"host" mapstructure:"host"`
	Port     int    `json:"port" mapstructure:"port"`
}

func Load(configFile string) (*ArcConfig, error) {
	arcConfig := getDefaultArcConfig()

	// set defaults in viper
	err := setDefaults(arcConfig)
	if err != nil {
		return nil, err
	}

	// set ENV config
	viper.SetEnvPrefix("ARC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// read app config from file into viper (overriding specified defaults)
	viper.AddConfigPath(configFile)
	err = viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	// unmarshall viper into arcConfig
	err = viper.Unmarshal(arcConfig)
	if err != nil {
		return nil, err
	}

	return arcConfig, nil
}

func setDefaults(defaultConfig *ArcConfig) error {
	defaultsMap := make(map[string]interface{})
	if err := mapstructure.Decode(defaultConfig, &defaultsMap); err != nil {
		err = fmt.Errorf("error occurred while setting defaults: %w", err)
		return err
	}

	for key, value := range defaultsMap {
		viper.SetDefault(key, value)
	}

	return nil
}
