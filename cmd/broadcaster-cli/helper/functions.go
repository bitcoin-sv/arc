package helper

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/lmittmann/tint"

	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

func CreateClient(auth *broadcaster.Auth, arcServer string, logger *slog.Logger) (broadcaster.ArcClient, error) {
	arcClient, err := broadcaster.GetArcClient(arcServer, auth)
	if err != nil {
		return nil, err
	}

	return broadcaster.NewHTTPBroadcaster(arcClient, logger)
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

func GetString(settingName string) string {
	setting := viper.GetString(settingName)
	if setting != "" {
		return setting
	}

	return getSettingFromEnvFile[string](settingName)
}

func GetInt(settingName string) int {
	setting := viper.GetInt(settingName)
	if setting != 0 {
		return setting
	}

	return getSettingFromEnvFile[int](settingName)
}

func GetUint64(settingName string) uint64 {
	setting := viper.GetUint64(settingName)
	if setting != 0 {
		return setting
	}

	return getSettingFromEnvFile[uint64](settingName)
}

func GetUint32(settingName string) uint32 {
	setting := viper.GetUint32(settingName)
	if setting != 0 {
		return setting
	}

	return getSettingFromEnvFile[uint32](settingName)
}

func GetInt64(settingName string) int64 {
	setting := viper.GetInt64(settingName)
	if setting != 0 {
		return setting
	}

	return getSettingFromEnvFile[int64](settingName)
}

func getSettingFromEnvFile[T any](settingName string) T {
	var result map[string]interface{}
	var nullValue T

	err := viper.Unmarshal(&result)
	if err != nil {
		return nullValue
	}

	value, ok := result[strings.ToLower(settingName)].(T)

	if !ok {
		return nullValue
	}

	return value
}

func GetBool(settingName string) bool {
	setting := viper.GetBool(settingName)
	if setting {
		return true
	}

	return getSettingFromEnvFile[bool](settingName)
}

func NewLogger(logLevel, logFormat string) *slog.Logger {
	var slogLevel slog.Level

	switch logLevel {
	case "INFO":
		slogLevel = slog.LevelInfo
	case "WARN":
		slogLevel = slog.LevelWarn
	case "ERROR":
		slogLevel = slog.LevelError
	case "DEBUG":
		slogLevel = slog.LevelDebug
	default:
		slogLevel = slog.LevelInfo
	}

	switch logFormat {
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
	case "text":
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
	case "tint":
		return slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slogLevel}))
	default:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
	}
}

func GetKeySetsFor(keys map[string]string, selectedKeys []string) (map[string]*keyset.KeySet, error) {
	keySets := map[string]*keyset.KeySet{}

	if len(keys) == 0 {
		return nil, errors.New("no keys given in configuration")
	}

	if len(selectedKeys) > 0 {
		for _, selectedKey := range selectedKeys {
			key, found := keys[selectedKey]
			if !found {
				return nil, fmt.Errorf("key not found: %s", selectedKey)
			}
			fundingKeySet, _, err := GetKeySetsXpriv(key)
			if err != nil {
				return nil, fmt.Errorf("failed to get selected key set %s: %v", selectedKey, err)
			}
			keySets[selectedKey] = fundingKeySet
		}
		return keySets, nil
	}

	for name, key := range keys {
		fundingKeySet, _, err := GetKeySetsXpriv(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get key set with name %s and value %s: %v", name, key, err)
		}
		keySets[name] = fundingKeySet
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

func GetSelectedKeys() ([]string, error) {
	var keys []string
	err := viper.UnmarshalKey("keys", &keys)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func GetSelectedKeySets() (map[string]*keyset.KeySet, error) {
	selectedKeys, err := GetSelectedKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get selected keys: %v", err)
	}

	keys, err := GetPrivateKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get private keys: %v", err)
	}

	if len(keys) == 0 {
		return nil, errors.New("no keys given in configuration")
	}

	return GetKeySetsFor(keys, selectedKeys)
}

func GetAllKeySets() (map[string]*keyset.KeySet, error) {
	keys, err := GetPrivateKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get private keys: %v", err)
	}

	if len(keys) == 0 {
		return nil, errors.New("no keys given in configuration")
	}
	keySets := map[string]*keyset.KeySet{}

	for name, key := range keys {
		fundingKeySet, _, err := GetKeySetsXpriv(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get key set with name %s and value %s: %v", name, key, err)
		}
		keySets[name] = fundingKeySet
	}

	return keySets, nil
}

func GetOrderedKeys[T any](keysMap map[string]T) []string {
	var keys []string

	for key := range keysMap {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}
