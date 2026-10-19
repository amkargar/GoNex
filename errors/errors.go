// Package errors defines transport-independent application errors.
package errors

import "fmt"

type Code string

const (
	NotFound     Code = "not_found"
	Conflict     Code = "conflict"
	Unavailable  Code = "unavailable"
	Invalid      Code = "invalid"
	Unauthorized Code = "unauthorized"
	Forbidden    Code = "forbidden"
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string                          { return e.Message }
func (e *Error) Unwrap() error                          { return e.Cause }
func New(code Code, message string) error               { return &Error{Code: code, Message: message} }
func Wrap(code Code, message string, cause error) error { return &Error{code, message, cause} }
func (e *Error) GoString() string                       { return fmt.Sprintf("application error (%s)", e.Code) }
