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
	refreshRate        time.Duration
}

func NewZMQHandler(ctx context.Context, zmqURL *url.URL, logger *slog.Logger) *ZMQHandler {
	return NewZMQHandlerWithRefreshRate(ctx, zmqURL, logger, 10*time.Second)
}
func NewZMQHandlerWithRefreshRate(ctx context.Context, zmqURL *url.URL, logger *slog.Logger, refreshRate time.Duration) *ZMQHandler {
	zmq := &ZMQHandler{
		address:            fmt.Sprintf("tcp://%s:%s", zmqURL.Hostname(), zmqURL.Port()),
		subscriptions:      make(map[string][]chan []string),
		addSubscription:    make(chan subscriptionRequest, 10),
		removeSubscription: make(chan subscriptionRequest, 10),
		logger:             logger.With(slog.String("module", "zmq-handler")),
		refreshRate:        refreshRate,
	}

	go zmq.start(ctx)

	return zmq
}

func (zmqHandler *ZMQHandler) start(ctx context.Context) {
	for {
		zmqHandler.socket = zmq4.NewSub(ctx, zmq4.WithID(zmq4.SocketIdentity("sub")))
		defer func() {
			if zmqHandler.connected {
				if err := zmqHandler.socket.Close(); err != nil {
					zmqHandler.logger.Error("failed to close zmq socket", slog.String("error", err.Error()))
				}
				zmqHandler.connected = false
			}
		}()

		if err := zmqHandler.socket.Dial(zmqHandler.address); err != nil {
			zmqHandler.err = err
			zmqHandler.logger.Error("Could not dial ZMQ", slog.String("address", zmqHandler.address), slog.String("error", err.Error()))
			zmqHandler.logger.Info("Attempting to re-establish ZMQ connection in ...", slog.Duration("refreshRate", zmqHandler.refreshRate))
			time.Sleep(zmqHandler.refreshRate)
			continue
		}

		zmqHandler.logger.Info("ZMQ: Connecting", slog.String("address", zmqHandler.address))

		for topic := range zmqHandler.subscriptions {
			if err := zmqHandler.socket.SetOption(zmq4.OptionSubscribe, topic); err != nil {
				zmqHandler.err = fmt.Errorf("%w", err)
				return
			}
			zmqHandler.logger.Info("ZMQ: Subscribed", slog.String("topic", topic))
		}

		err := zmqHandler.checkZMQHandlerCases(ctx)
		if err != nil {
			return
		}

		zmqHandler.checkConnection()
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

func (zmqHandler *ZMQHandler) checkZMQHandlerCases(ctx context.Context) error {
OUT:
	for {
		select {
		case <-ctx.Done():
			zmqHandler.logger.Info("ZMQ: Context done, exiting")
			return errors.New("context done")
		case req := <-zmqHandler.addSubscription:
			zmqHandler.addSubscriptionFn(req)

		case req := <-zmqHandler.removeSubscription:
			zmqHandler.removeSubscriptionFn(req)

		default:
			err := zmqHandler.receiveMessage()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				zmqHandler.logger.Error("zmqHandler.socket.Recv()", slog.String("error", err.Error()))
				break OUT
			}
		}
	}
	return nil
}

func (zmqHandler *ZMQHandler) addSubscriptionFn(req subscriptionRequest) {
	if err := zmqHandler.socket.SetOption(zmq4.OptionSubscribe, req.topic); err != nil {
		zmqHandler.logger.Error("ZMQ: Failed to subscribe", slog.String("topic", req.topic))
	} else {
		zmqHandler.logger.Info("ZMQ: Subscribed", slog.String("topic", req.topic))
	}

	subscribers := zmqHandler.subscriptions[req.topic]
	subscribers = append(subscribers, req.ch)
	zmqHandler.subscriptions[req.topic] = subscribers
}

func (zmqHandler *ZMQHandler) removeSubscriptionFn(req subscriptionRequest) {
	subscribers := zmqHandler.subscriptions[req.topic]
	for i, subscriber := range subscribers {
		if subscriber == req.ch {
			subscribers = append(subscribers[:i], subscribers[i+1:]...)
			zmqHandler.logger.Info("Removed subscription", slog.String("topic", req.topic))
			break
		}
	}
	zmqHandler.subscriptions[req.topic] = subscribers
}

func (zmqHandler *ZMQHandler) receiveMessage() error {
	msg, err := zmqHandler.socket.Recv()
	if err != nil {
		return err
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
	return nil
}

func (zmqHandler *ZMQHandler) checkConnection() {
	if zmqHandler.connected {
		if err := zmqHandler.socket.Close(); err != nil {
			zmqHandler.logger.Error("failed to close zmq socket", slog.String("error", err.Error()))
		}
		zmqHandler.connected = false
	}
	zmqHandler.logger.Info("Attempting to re-establish ZMQ connection in ...", slog.Duration("refreshRate", zmqHandler.refreshRate))
	time.Sleep(zmqHandler.refreshRate)
}
