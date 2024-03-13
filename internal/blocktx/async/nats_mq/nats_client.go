package nats_mq

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"time"
)

func NewNatsClient(natsURL string) (NatsClient, error) {
	var nc *nats.Conn
	var err error

	nc, err = nats.Connect(natsURL)
	if err == nil {
		return nc, nil
	}

	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, err
	}
	defer ec.Close()

	// Try to reconnect in intervals
	i := 0
	for range time.NewTicker(2 * time.Second).C {
		nc, err = nats.Connect(natsURL)
		if err != nil && i >= connectionTries {
			return nil, fmt.Errorf("failed to connect to NATS server: %v", err)
		}

		if err == nil {
			break
		}
		i++
	}

	return nc, nil
}
