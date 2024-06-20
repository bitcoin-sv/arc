package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

func Load(configFileDirs ...string) (*ArcConfig, error) {
	arcConfig := getDefaultArcConfig()

	err := setDefaults(arcConfig)
	if err != nil {
		return nil, err
	}

	viper.SetEnvPrefix("ARC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	err = overrideWithFiles(configFileDirs...)
	if err != nil {
		return nil, err
	}

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

func overrideWithFiles(configFileDirs ...string) error {
	if len(configFileDirs) == 0 || configFileDirs[0] == "" {
		return nil
	}

	for _, path := range configFileDirs {
		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("config path: %s does not exist", path)
			}
			return err
		}
		if !stat.IsDir() {
			return fmt.Errorf("config path: %s should be a directory", path)
		}

		viper.AddConfigPath(path)
	}

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	return nil
}
