package helper

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/bitcoin-sv/arc//internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/keyset"
	"github.com/spf13/viper"
)

func CreateClient(auth *broadcaster.Auth, arcServer string) (broadcaster.ArcClient, error) {
	arcServerUrl, err := url.Parse(arcServer)
	if err != nil {
		return nil, errors.New("arcUrl is not a valid url")
	}
	return broadcaster.NewHTTPBroadcaster(arcServerUrl.String(), auth), nil
}

func GetKeySetsKeyFile(keyFile string) (fundingKeySet *keyset.KeySet, receivingKeySet *keyset.KeySet, err error) {
	var extendedBytes []byte
	extendedBytes, err = os.ReadFile(keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, errors.New("arc.key not found. Please create this file with the xpriv you want to use")
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

	var result map[string]interface{}

	err := viper.ReadInConfig()
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return "", nil
		}
		return "", err
	}

	err = viper.Unmarshal(&result)
	if err != nil {
		return "", err
	}

	value, ok := result[strings.ToLower(settingName)].(string)
	if !ok {
		return "", nil
	}

	return value, nil
}

func GetInt(settingName string) (int, error) {

	setting := viper.GetInt(settingName)
	if setting != 0 {
		return setting, nil
	}

	var result map[string]interface{}

	err := viper.ReadInConfig()
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return 0, nil
		}
		return 0, err
	}

	err = viper.Unmarshal(&result)
	if err != nil {
		return 0, err
	}

	value, ok := result[settingName].(int)

	if !ok {
		return 0, nil
	}

	return value, nil
}

func GetBool(settingName string) (bool, error) {

	setting := viper.GetBool(settingName)
	if setting {
		return true, nil
	}

	var result map[string]interface{}

	err := viper.ReadInConfig()
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return false, nil
		}
		return false, err
	}

	err = viper.Unmarshal(&result)
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
