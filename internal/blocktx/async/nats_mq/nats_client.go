package nats_mq

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const connectionTries = 5

func NewNatsClient(natsURL string) (*nats.Conn, error) {
	var nc *nats.Conn
	var err error

	nc, err = nats.Connect(natsURL)
	if err == nil {
		return nc, nil
	}

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
