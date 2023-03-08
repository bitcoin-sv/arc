package callbacker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/callbacker/store"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

type Callbacker struct {
	logger *gocore.Logger
	store  store.Store
	ticker *time.Ticker
}

var logLevel, _ = gocore.Config().Get("logLevel")
var logger = gocore.Log("callbacker", gocore.NewLogLevelFromString(logLevel))

// New creates a new callback worker
func New(s store.Store) (*Callbacker, error) {
	if s == nil {
		return nil, fmt.Errorf("store is nil")
	}

	return &Callbacker{
		logger: logger,
		store:  s,
	}, nil
}

func (c *Callbacker) Start() {
	c.ticker = time.NewTicker(30 * time.Second)
	go func() {
		for range c.ticker.C {
			err := c.sendCallbacks()
			if err != nil {
				c.logger.Errorf("failed to send callbacks: %v", err)
			}
		}
	}()
}

func (c *Callbacker) Stop() {
	c.ticker.Stop()
}

func (c *Callbacker) AddCallback(ctx context.Context, callback *callbacker_api.Callback) (string, error) {
	key, err := c.store.Set(ctx, callback)
	if err != nil {
		return "", err
	}

	// try to send the callback the first time, in the background, we don't want to wait for the timeout
	go func() {
		err = c.sendCallback(key, callback)
		if err != nil {
			c.logger.Errorf("failed to send callback: %v", err)
		}
	}()

	return key, nil
}

func (c *Callbacker) sendCallbacks() error {
	callbacks, err := c.store.GetExpired(context.Background())
	if err != nil {
		return err
	}

	if len(callbacks) > 0 {
		c.logger.Infof("sending %d callbacks", len(callbacks))

		for key, callback := range callbacks {
			c.logger.Debugf("sending callback: %s => %s", key, callback.Url)
			err = c.sendCallback(key, callback)
			if err != nil {
				c.logger.Errorf("failed to send callback: %v", err)
			}
		}
	}

	return nil
}

func (c *Callbacker) sendCallback(key string, callback *callbacker_api.Callback) error {
	txId := utils.ReverseAndHexEncodeSlice(callback.Hash)
	c.logger.Infof("sending callback: %s => %s", txId, callback.Url)

	statusString := metamorph_api.Status(callback.Status).String()
	blockHash := ""
	if callback.BlockHash != nil {
		blockHash = utils.ReverseAndHexEncodeSlice(callback.BlockHash)
	}
	status := &api.TransactionStatus{
		BlockHash:   &blockHash,
		BlockHeight: &callback.BlockHeight,
		TxStatus:    &statusString,
		Txid:        txId,
	}
	statusBytes, err := json.Marshal(status)
	if err != nil {
		return err
	}
	statusBuffer := bytes.NewBuffer(statusBytes)

	var request *http.Request
	request, err = http.NewRequest("POST", callback.Url, statusBuffer)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	if callback.Token != "" {
		request.Header.Set("Authorization", "Bearer "+callback.Token)
	}

	// default http client
	httpClient := http.Client{}
	httpClient.Timeout = 5 * time.Second

	var response *http.Response
	response, err = httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		err = c.store.Del(context.Background(), key)
	} else {
		err = c.store.UpdateExpiry(context.Background(), key)
	}

	return err
}
