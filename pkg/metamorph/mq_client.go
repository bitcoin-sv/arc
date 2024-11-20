package metamorph

import (
	"context"

	"google.golang.org/protobuf/proto"
)

const (
	SubmitTxTopic = "submit-tx"
)

type MessageQueueClient interface {
	PublishMarshal(ctx context.Context, topic string, m proto.Message) error
	Shutdown()
}
