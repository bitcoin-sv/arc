package config

import (
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"os"
	"strings"
)

var (
	ErrConfigFailedToSetDefaults = errors.New("error occurred while setting defaults")
	ErrConfigPath                = errors.New("config path error")
)

func Load(configFileDirs ...string) (*ArcConfig, error) {
	arcConfig := getDefaultArcConfig()

	err := setDefaults(arcConfig)
	if err != nil {
		return nil, err
	}

	err = overrideWithFiles(configFileDirs...)
	if err != nil {
		return nil, err
	}

	viper.SetEnvPrefix("ARC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	err = viper.Unmarshal(arcConfig)
	if err != nil {
		return nil, err
	}

	return arcConfig, nil
}

func setDefaults(defaultConfig *ArcConfig) error {
	defaultsMap := make(map[string]interface{})

	if err := mapstructure.Decode(defaultConfig, &defaultsMap); err != nil {
		err = errors.Join(ErrConfigFailedToSetDefaults, err)
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
				return errors.Join(ErrConfigPath, fmt.Errorf("path: %s does not exist", path))
			}
			return err
		}
		if !stat.IsDir() {
			return errors.Join(ErrConfigPath, fmt.Errorf("path: %s should be a directory", path))
		}

		viper.AddConfigPath(path)
	}

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	return nil
}
