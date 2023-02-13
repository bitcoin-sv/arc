package metamorph

import (
	"encoding/json"
	"time"

	gcutils "github.com/ordishs/gocore/utils"
)

type ProcessorResponseLog struct {
	T      int64  `json:"t"`
	Status string `json:"status"`
	Source string `json:"source,omitempty"`
	Info   string `json:"info,omitempty"`
}

func (p *ProcessorResponseLog) MarshalJSON() ([]byte, error) {
	type Alias ProcessorResponseLog
	return json.Marshal(&struct {
		T string `json:"t"`
		*Alias
	}{
		T:     gcutils.HumanTimeUnit(time.Duration(p.T)),
		Alias: (*Alias)(p),
	})
}
