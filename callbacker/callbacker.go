package callbacker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/callbacker/store"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/ordishs/go-utils"
)

const (
	logLevelDefault              = slog.LevelInfo
	sendCallbacksIntervalDefault = 30 * time.Second
)

type Callbacker struct {
	logger                *slog.Logger
	store                 store.Store
	ticker                *time.Ticker
	sendCallbacksInterval time.Duration
	shutdownCompleteStart chan struct{}
	shutdown              chan struct{}
}

func WithLogger(logger *slog.Logger) func(*Callbacker) {
	return func(p *Callbacker) {
		p.logger = logger.With(slog.String("service", "clb"))
	}
}

func WithSendCallbacksInterval(d time.Duration) func(callbacker *Callbacker) {
	return func(p *Callbacker) {
		p.sendCallbacksInterval = d
	}
}

type Option func(f *Callbacker)

// New creates a new callback worker.
func New(s store.Store, opts ...Option) (*Callbacker, error) {
	if s == nil {
		return nil, fmt.Errorf("store is nil")
	}

	c := &Callbacker{
		store:                 s,
		logger:                slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "clb")),
		sendCallbacksInterval: sendCallbacksIntervalDefault,
		shutdown:              make(chan struct{}, 1),
		shutdownCompleteStart: make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (c *Callbacker) Start() {
	c.ticker = time.NewTicker(c.sendCallbacksInterval)
	go func() {
		defer func() {
			c.shutdownCompleteStart <- struct{}{}
		}()
		for {
			select {
			case <-c.ticker.C:
				err := c.sendCallbacks()
				if err != nil {
					c.logger.Error("failed to send callbacks", slog.String("err", err.Error()))
				}
			case <-c.shutdown:
				return
			}
		}
	}()
}

func (c *Callbacker) Stop() {
	c.ticker.Stop()
	c.shutdown <- struct{}{}

	// wait until shutdown is complete
	<-c.shutdownCompleteStart
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
			c.logger.Error("failed to send callback", slog.String("err", err.Error()))
		}
	}()

	return key, nil
}

func (c *Callbacker) sendCallbacks() error {
	callbacks, err := c.store.GetExpired(context.Background())
	if err != nil {
		return err
	}

	if len(callbacks) == 0 {
		return nil
	}

	c.logger.Info("sending callbacks", slog.Int("number", len(callbacks)))

	for key, callback := range callbacks {
		c.logger.Debug("sending callback", slog.String("callbackID", key), slog.String("url", callback.GetUrl()))
		err = c.sendCallback(key, callback)
		if err != nil {
			c.logger.Error("failed to send callback", slog.String("err", err.Error()))
		}
	}

	return nil
}

func (c *Callbacker) sendCallback(key string, callback *callbacker_api.Callback) error {
	txId := utils.ReverseAndHexEncodeSlice(callback.GetHash())

	statusString := metamorph_api.Status(callback.GetStatus()).String()
	blockHash := ""
	if callback.BlockHash != nil {
		blockHash = utils.ReverseAndHexEncodeSlice(callback.GetBlockHash())
	}

	c.logger.Info("sending callback for transaction", slog.String("token", callback.GetToken()), slog.String("hash", txId), slog.String("url", callback.GetUrl()), slog.Uint64("block height", callback.GetBlockHeight()), slog.String("block hash", blockHash))

	status := &api.TransactionStatus{
		BlockHash:   &blockHash,
		BlockHeight: &callback.BlockHeight,
		TxStatus:    &statusString,
		Txid:        txId,
		Timestamp:   time.Now(),
	}
	statusBytes, err := json.Marshal(status)
	if err != nil {
		return err
	}
	statusBuffer := bytes.NewBuffer(statusBytes)

	var request *http.Request
	request, err = http.NewRequest("POST", callback.GetUrl(), statusBuffer)
	if err != nil {
		return errors.Join(err, fmt.Errorf("failed to post callback for transaction id %s", txId))
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	if callback.GetToken() != "" {
		request.Header.Set("Authorization", "Bearer "+callback.GetToken())
	}

	// default http client
	httpClient := http.Client{}
	httpClient.Timeout = 5 * time.Second

	var response *http.Response
	response, err = httpClient.Do(request)
	if err != nil {
		errUpdateExpiry := c.store.UpdateExpiry(context.Background(), key)
		if errUpdateExpiry != nil {
			return errors.Join(errUpdateExpiry, fmt.Errorf("failed to update expiry of key %s after http request failed: %v", key, err))
		}

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
