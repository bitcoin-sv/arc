package processor_response

import "github.com/TAAL-GmbH/arc/metamorph/metamorph_api"

type ProcessorResponseLog struct {
	DeltaT int64                `json:"delta_t"`
	Status metamorph_api.Status `json:"status"`
	Source string               `json:"source,omitempty"`
	Info   string               `json:"info,omitempty"`
}
