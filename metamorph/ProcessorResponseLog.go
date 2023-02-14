package metamorph

type ProcessorResponseLog struct {
	DeltaT int64  `json:"delta_t"`
	Status string `json:"status"`
	Source string `json:"source,omitempty"`
	Info   string `json:"info,omitempty"`
}
