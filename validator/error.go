package validator

import (
	"fmt"

	"github.com/TAAL-GmbH/arc/api"
)

type Error struct {
	Err            error
	ArcErrorStatus api.ErrorCode
}

func NewError(err error, status api.ErrorCode) *Error {
	return &Error{
		Err:            err,
		ArcErrorStatus: status,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("arc error %d: %s", e.ArcErrorStatus, e.Err.Error())
}
