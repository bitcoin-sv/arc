package metamorph

import (
	"context"

	"github.com/TAAL-GmbH/arc/metamorph/processor_response"
	"github.com/TAAL-GmbH/arc/metamorph/store"
)

type ProcessorRequest struct {
	*store.StoreData
	ResponseChannel chan processor_response.StatusAndError
	ctx             context.Context
}

func NewProcessorRequest(ctx context.Context, s *store.StoreData, responseChannel chan processor_response.StatusAndError) *ProcessorRequest {
	return &ProcessorRequest{
		s,
		responseChannel,
		ctx,
	}
}
