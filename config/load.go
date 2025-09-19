package config

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"
)

var (
	ErrConfigFailedToSetDefaults = errors.New("error occurred while setting defaults")
	ErrConfigPath                = errors.New("config path error")
)

func Load(configFiles ...string) (*ArcConfig, error) {
	arcConfig := getDefaultArcConfig()

	err := setDefaults(arcConfig)
	if err != nil {
		return nil, err
	}

	err = overrideWithFiles(configFiles...)
	if err != nil {
		return nil, err
	}

	viper.SetEnvPrefix("ARC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	err = viper.Unmarshal(arcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	if arcConfig.Common.Tracing != nil {
		tracingAttributes := make([]attribute.KeyValue, len(arcConfig.Common.Tracing.Attributes))
		index := 0
		for key, value := range arcConfig.Common.Tracing.Attributes {
			tracingAttributes[index] = attribute.String(key, value)
			index++
		}

		if len(tracingAttributes) > 0 {
			arcConfig.Common.Tracing.KeyValueAttributes = tracingAttributes
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

func overrideWithFiles(files ...string) error {
	if len(files) == 0 || files[0] == "" {
		return nil
	}

	for _, f := range files {
		viper.SetConfigFile(f)
		err := viper.MergeInConfig()
		if err != nil {
			return errors.Join(ErrConfigPath, err)
		}

		log.Default().Printf("Merged config from:%s", viper.ConfigFileUsed())
	}

	return nil
}
