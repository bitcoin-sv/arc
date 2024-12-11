package validator

import (
	"fmt"
	"github.com/bitcoin-sv/arc/pkg/api"
)

type Error struct {
	Err            error
	ArcErrorStatus api.StatusCode
}

func NewError(err error, status api.StatusCode) *Error {
	return &Error{
		Err:            err,
		ArcErrorStatus: status,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("arc error %d: %s", e.ArcErrorStatus, e.Err.Error())
}
