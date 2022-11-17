package validator

import (
	"fmt"

	"github.com/TAAL-GmbH/mapi"
)

type Error struct {
	Err             error
	MapiErrorStatus mapi.ErrStatus
}

func NewError(err error, status mapi.ErrStatus) *Error {
	return &Error{
		Err:             err,
		MapiErrorStatus: status,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("mapi error %d: %s", e.MapiErrorStatus, e.Err.Error())
}
