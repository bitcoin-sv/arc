package metamorph

import (
	"context"

	"github.com/TAAL-GmbH/arc/metamorph/store"
)

type ProcessorRequest struct {
	*store.StoreData
	ResponseChannel chan StatusAndError
	ctx             context.Context
}

func NewProcessorRequest(ctx context.Context, s *store.StoreData, responseChannel chan StatusAndError) *ProcessorRequest {
	return &ProcessorRequest{
		s,
		responseChannel,
		ctx,
	}
}
