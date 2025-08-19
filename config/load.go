package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"
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
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if arcConfig.Tracing != nil {
		tracingAttributes := make([]attribute.KeyValue, len(arcConfig.Tracing.Attributes))
		index := 0
		for key, value := range arcConfig.Tracing.Attributes {
			tracingAttributes[index] = attribute.String(key, value)
			index++
		}

		if len(tracingAttributes) > 0 {
			arcConfig.Tracing.KeyValueAttributes = tracingAttributes
		}
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
