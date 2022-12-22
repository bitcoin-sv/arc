package metamorph

import "github.com/TAAL-GmbH/arc/metamorph/metamorph_api"

type SendStatusForTransactionCall struct {
	HashStr string
	Status  metamorph_api.Status
	Err     error
}

type ProcessorMock struct {
	Stats                         *ProcessorStats
	ProcessTransactionCalls       []*ProcessorRequest
	SendStatusForTransactionCalls []*SendStatusForTransactionCall
}

func NewProcessorMock() *ProcessorMock {
	return &ProcessorMock{}
}

func (p ProcessorMock) LoadUnseen() {}

func (p ProcessorMock) ProcessTransaction(req *ProcessorRequest) {
	p.ProcessTransactionCalls = append(p.ProcessTransactionCalls, req)
}

func (p ProcessorMock) SendStatusForTransaction(hashStr string, status metamorph_api.Status, err error) bool {
	p.SendStatusForTransactionCalls = append(p.SendStatusForTransactionCalls, &SendStatusForTransactionCall{
		HashStr: hashStr,
		Status:  status,
		Err:     err,
	})
	return true
}

func (p ProcessorMock) GetStats() *ProcessorStats {
	return p.Stats
}
