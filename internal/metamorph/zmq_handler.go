package metamorph

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"github.com/go-zeromq/zmq4"
)

type ZMQHandler struct {
	address            string
	socket             zmq4.Socket
	connected          bool
	err                error
	subscriptions      map[string][]chan []string
	addSubscription    chan subscriptionRequest
	removeSubscription chan subscriptionRequest
	logger             *slog.Logger
}

func NewZMQHandler(ctx context.Context, zmqURL *url.URL, logger *slog.Logger) *ZMQHandler {
	zmq := &ZMQHandler{
		address:            fmt.Sprintf("tcp://%s:%s", zmqURL.Hostname(), zmqURL.Port()),
		subscriptions:      make(map[string][]chan []string),
		addSubscription:    make(chan subscriptionRequest, 10),
		removeSubscription: make(chan subscriptionRequest, 10),
		logger:             logger.With(slog.String("module", "zmq-handler")),
	}

	go zmq.start(ctx)

	return zmq
}

func (zmqHandler *ZMQHandler) start(ctx context.Context) {
	for {
		zmqHandler.socket = zmq4.NewSub(ctx, zmq4.WithID(zmq4.SocketIdentity("sub")))
		defer func() {
			if zmqHandler.connected {
				zmqHandler.socket.Close()
				zmqHandler.connected = false
			}
		}()

		if err := zmqHandler.socket.Dial(zmqHandler.address); err != nil {
			zmqHandler.err = err
			zmqHandler.logger.Error("Could not dial ZMQ", slog.String("address", zmqHandler.address), slog.String("error", err.Error()))
			zmqHandler.logger.Info("Attempting to re-establish ZMQ connection in 10 seconds...")
			time.Sleep(10 * time.Second)
			continue
		}

		zmqHandler.logger.Info("ZMQ: Connecting", slog.String("address", zmqHandler.address))

		for topic := range zmqHandler.subscriptions {
			if err := zmqHandler.socket.SetOption(zmq4.OptionSubscribe, topic); err != nil {
				zmqHandler.err = fmt.Errorf("%+v", err)
				return
			}
			zmqHandler.logger.Info("ZMQ: Subscribed", slog.String("topic", topic))
		}

	OUT:
		for {
			select {
			case <-ctx.Done():
				zmqHandler.logger.Info("ZMQ: Context done, exiting")
				return
			case req := <-zmqHandler.addSubscription:
				if err := zmqHandler.socket.SetOption(zmq4.OptionSubscribe, req.topic); err != nil {
					zmqHandler.logger.Error("ZMQ: Failed to subscribe", slog.String("topic", req.topic))
				} else {
					zmqHandler.logger.Info("ZMQ: Subscribed", slog.String("topic", req.topic))
				}

				subscribers := zmqHandler.subscriptions[req.topic]
				subscribers = append(subscribers, req.ch)

				zmqHandler.subscriptions[req.topic] = subscribers

			case req := <-zmqHandler.removeSubscription:
				subscribers := zmqHandler.subscriptions[req.topic]
				for i, subscriber := range subscribers {
					if subscriber == req.ch {
						subscribers = append(subscribers[:i], subscribers[i+1:]...)
						zmqHandler.logger.Info("Removed subscription", slog.String("topic", req.topic))
						break
					}
				}
				zmqHandler.subscriptions[req.topic] = subscribers

			default:
				msg, err := zmqHandler.socket.Recv()
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					zmqHandler.logger.Error("zmqHandler.socket.Recv()", slog.String("error", err.Error()))
					break OUT
				}

				if !zmqHandler.connected {
					zmqHandler.connected = true
					zmqHandler.logger.Info("ZMQ: Connection observed", slog.String("address", zmqHandler.address))
				}

				subscribers := zmqHandler.subscriptions[string(msg.Frames[0])]

				sequence := "N/A"

				if len(msg.Frames) > 2 && len(msg.Frames[2]) == 4 {
					s := binary.LittleEndian.Uint32(msg.Frames[2])
					sequence = strconv.FormatInt(int64(s), 10)
				}

				for _, subscriber := range subscribers {
					subscriber <- []string{string(msg.Frames[0]), hex.EncodeToString(msg.Frames[1]), sequence}
				}
			}
		}

		if zmqHandler.connected {
			zmqHandler.socket.Close()
			zmqHandler.connected = false
		}
		zmqHandler.logger.Info("Attempting to re-establish ZMQ connection in 10 seconds...")
		time.Sleep(10 * time.Second)
	}
}

func (zmqHandler *ZMQHandler) Subscribe(topic string, ch chan []string) error {
	if !contains(allowedTopics, topic) {
		return fmt.Errorf("topic must be %+v, received %q", allowedTopics, topic)
	}

	zmqHandler.addSubscription <- subscriptionRequest{
		topic: topic,
		ch:    ch,
	}

	return nil
}

func (zmqHandler *ZMQHandler) Unsubscribe(topic string, ch chan []string) error {
	if !contains(allowedTopics, topic) {
		return fmt.Errorf("topic must be %+v, received %q", allowedTopics, topic)
	}

	zmqHandler.removeSubscription <- subscriptionRequest{
		topic: topic,
		ch:    ch,
	}

	return nil
}
