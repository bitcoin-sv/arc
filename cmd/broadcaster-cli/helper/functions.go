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

func CreateClient(auth *broadcaster.Auth, isDryRun bool, isAPIClient bool) (broadcaster.ClientI, error) {
	var client broadcaster.ClientI
	var err error
	if isDryRun {
		client = broadcaster.NewDryRunClient()
	} else if isAPIClient {

		arcServer := viper.GetString("api-url")
		if arcServer == "" {
			arcServer = viper.GetString("broadcaster.apiURL")
			if arcServer == "" {
				return nil, errors.New("arcUrl not found in config")
			}
		}

		arcServerUrl, err := url.Parse(arcServer)
		if err != nil {
			return nil, errors.New("arcUrl is not a valid url")
		}

		// create a http connection to the arc node
		client = broadcaster.NewHTTPBroadcaster(arcServerUrl.String(), auth)
	} else {
		addresses := viper.GetString("metamorph.dialAddr")
		if addresses == "" {
			return nil, errors.New("metamorph.dialAddr not found in config")
		}
		client, err = broadcaster.NewMetamorphBroadcaster(addresses)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
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
