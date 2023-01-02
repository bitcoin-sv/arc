package metamorph

import "github.com/TAAL-GmbH/arc/metamorph/store"

type ProcessorRequest struct {
	*store.StoreData
	ResponseChannel chan StatusAndError
}

func NewProcessorRequest(s *store.StoreData, responseChannel chan StatusAndError) *ProcessorRequest {
	return &ProcessorRequest{
		s,
		responseChannel,
	}
}
