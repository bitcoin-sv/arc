package helper

import (
	"errors"
	"net/url"
	"os"
	"strings"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/lib/keyset"
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

func GetNewKeySets() (fundingKeySet *keyset.KeySet, receivingKeySet *keyset.KeySet, err error) {
	fundingKeySet, err = keyset.New()
	if err != nil {
		return nil, nil, err
	}
	receivingKeySet = fundingKeySet

	return fundingKeySet, receivingKeySet, err
}

func GetString(settingName string) string {

	setting := viper.GetString(settingName)
	if setting != "" {
		return setting
	}

	viper.AddConfigPath(".")
	var result map[string]interface{}
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		return ""
	}

	err := viper.Unmarshal(&result)
	if err != nil {
		return ""
	}

	value, ok := result[strings.ToLower(settingName)].(string)
	if !ok {
		return ""
	}

	return value
}

func GetInt(settingName string) int {

	setting := viper.GetInt(settingName)
	if setting != 0 {
		return setting
	}

	viper.AddConfigPath(".")
	var result map[string]interface{}
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		return 0
	}

	err := viper.Unmarshal(&result)
	if err != nil {
		return 0
	}

	value, ok := result[settingName].(int)

	if !ok {
		return 0
	}

	return value
}

func GetBool(settingName string) bool {

	setting := viper.GetBool(settingName)
	if setting {
		return true
	}

	viper.AddConfigPath(".")
	var result map[string]interface{}
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		return false
	}

	err := viper.Unmarshal(&result)
	if err != nil {
		return false
	}

	value, ok := result[settingName].(bool)
	if !ok {
		return false
	}

	return value
}
