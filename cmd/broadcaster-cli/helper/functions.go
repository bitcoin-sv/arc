package helper

import (
	"fmt"
	"github.com/lmittmann/tint"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/spf13/viper"
)

func CreateClient(auth *broadcaster.Auth, arcServer string) (broadcaster.ArcClient, error) {
	return broadcaster.NewHTTPBroadcaster(arcServer, auth)
}

func GetKeySetsKeyFile(keyFile string) (fundingKeySet *keyset.KeySet, receivingKeySet *keyset.KeySet, err error) {
	var extendedBytes []byte
	extendedBytes, err = os.ReadFile(keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("key file %s not found. Please create this file with the xpriv you want to use", keyFile)
		}
		return nil, nil, err
	}
	xpriv := strings.TrimRight(strings.TrimSpace((string)(extendedBytes)), "\n")

	return GetKeySetsXpriv(xpriv)
}

func GetKeySetsXpriv(xpriv string) (fundingKeySet *keyset.KeySet, receivingKeySet *keyset.KeySet, err error) {
	fundingKeySet, err = keyset.NewFromExtendedKeyStr(xpriv, "0/0")
	if err != nil {
		return nil, nil, err
	}

	receivingKeySet, err = keyset.NewFromExtendedKeyStr(xpriv, "0/1")
	if err != nil {
		return nil, nil, err
	}

	return fundingKeySet, receivingKeySet, err
}

func GetString(settingName string) (string, error) {

	setting := viper.GetString(settingName)
	if setting != "" {
		return setting, nil
	}

	return getSettingFromEnvFile[string](settingName)
}

func GetInt(settingName string) (int, error) {

	setting := viper.GetInt(settingName)
	if setting != 0 {
		return setting, nil
	}

	return getSettingFromEnvFile[int](settingName)
}

func GetUint64(settingName string) (uint64, error) {

	setting := viper.GetUint64(settingName)
	if setting != 0 {
		return setting, nil
	}

	return getSettingFromEnvFile[uint64](settingName)
}

func GetInt64(settingName string) (int64, error) {

	setting := viper.GetInt64(settingName)
	if setting != 0 {
		return setting, nil
	}

	return getSettingFromEnvFile[int64](settingName)
}

func getSettingFromEnvFile[T any](settingName string) (T, error) {
	var result map[string]interface{}

	var nullValue T

	err := viper.Unmarshal(&result)
	if err != nil {
		return nullValue, err
	}

	value, ok := result[strings.ToLower(settingName)].(T)

	if !ok {
		return nullValue, nil
	}

	return value, nil
}

func GetBool(settingName string) (bool, error) {

	setting := viper.GetBool(settingName)
	if setting {
		return true, nil
	}

	var result map[string]interface{}

	err := viper.Unmarshal(&result)
	if err != nil {
		return false, err
	}

	valueString, ok := result[settingName].(string)
	if !ok {
		return false, nil
	}

	boolValue, err := strconv.ParseBool(valueString)
	if err != nil {
		return false, err
	}

	return boolValue, nil
}

func GetLogger() *slog.Logger {
	return slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug}))
}

func GetKeySets(keys map[string]string, selectedKeys []string) ([]*keyset.KeySet, error) {
	var keySets []*keyset.KeySet

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys given in configuration")
	}

	if len(selectedKeys) > 0 {
		for _, selectedKey := range selectedKeys {
			key, found := keys[selectedKey]
			if !found {
				return nil, fmt.Errorf("key not found: %s", selectedKey)
			}
			fundingKeySet, _, err := GetKeySetsXpriv(key)
			if err != nil {
				return nil, fmt.Errorf("failed to get key sets: %v", err)
			}
			keySets = append(keySets, fundingKeySet)
		}
	} else {
		for _, key := range keys {
			fundingKeySet, _, err := GetKeySetsXpriv(key)
			if err != nil {
				return nil, fmt.Errorf("failed to get key sets: %v", err)
			}
			keySets = append(keySets, fundingKeySet)
		}
	}

	return keySets, nil
}

func GetPrivateKeys() (map[string]string, error) {
	var keys map[string]string
	err := viper.UnmarshalKey("privateKeys", &keys)
	if err != nil {
		return nil, err
	}
	return keys, nil
}
