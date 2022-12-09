package validator

import (
	"fmt"

	"github.com/TAAL-GmbH/arc"
)

type Error struct {
	Err            error
	ArcErrorStatus arc.ErrorCode
}

func NewError(err error, status arc.ErrorCode) *Error {
	return &Error{
		Err:            err,
		ArcErrorStatus: status,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("arc error %d: %s", e.ArcErrorStatus, e.Err.Error())
}
