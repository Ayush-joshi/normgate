// Package errors defines stable transport-independent failure categories.
package errors

import "errors"

type Code string

const (
	Validation     Code = "ng.validation.invalid_input"
	Authentication Code = "ng.authentication.unauthenticated"
	Policy         Code = "ng.policy.unavailable"
	Enforcement    Code = "ng.enforcement.denied"
	Storage        Code = "ng.storage.unavailable"
	Upstream       Code = "ng.upstream.unavailable"
	Internal       Code = "ng.internal.failure"
)

type Error struct {
	Code  Code
	cause error
}

func New(code Code, cause error) *Error { return &Error{Code: code, cause: cause} }
func (e *Error) Error() string          { return string(e.Code) }
func (e *Error) Unwrap() error          { return e.cause }
func CodeOf(err error) Code {
	var typed *Error
	if errors.As(err, &typed) && typed != nil {
		return typed.Code
	}
	return Internal
}
