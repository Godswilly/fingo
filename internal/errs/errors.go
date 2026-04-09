package errs

import (
	"errors"
	"fmt"
	"net/http"
)

type Layer string

const (
	LayerDomain         Layer = "domain"
	LayerApplication    Layer = "application"
	LayerInfrastructure Layer = "infrastructure"
)

type Code string

const (
	CodeInvalidArgument    Code = "invalid_argument"
	CodeNotFound           Code = "not_found"
	CodeConflict           Code = "conflict"
	CodeInvariantViolation Code = "invariant_violation"
	CodeUnauthenticated    Code = "unauthenticated"
	CodePermissionDenied   Code = "permission_denied"
	CodeTimeout            Code = "timeout"
	CodeUnavailable        Code = "unavailable"
	CodeInternal           Code = "internal"
)

type Error struct {
	Layer   Layer
	Code    Code
	Op      string
	Message string
	Err     error
}

func (e *Error) Error() string {
	base := fmt.Sprintf("%s:%s", e.Layer, e.Code)
	if e.Op != "" {
		base = e.Op + ": " + base
	}
	if e.Message != "" {
		base += ": " + e.Message
	}
	if e.Err != nil {
		base += ": " + e.Err.Error()
	}
	return base
}

func (e *Error) Unwrap() error {
	return e.Err
}

func E(layer Layer, code Code, op, message string, err error) error {
	return &Error{
		Layer:   layer,
		Code:    code,
		Op:      op,
		Message: message,
		Err:     err,
	}
}

func Domain(code Code, op, message string, err error) error {
	return E(LayerDomain, code, op, message, err)
}

func Application(code Code, op, message string, err error) error {
	return E(LayerApplication, code, op, message, err)
}

func Infrastructure(code Code, op, message string, err error) error {
	return E(LayerInfrastructure, code, op, message, err)
}

func As(err error) (*Error, bool) {
	var target *Error
	ok := errors.As(err, &target)
	return target, ok
}

func IsCode(err error, code Code) bool {
	typed, ok := As(err)
	return ok && typed.Code == code
}

func IsLayer(err error, layer Layer) bool {
	typed, ok := As(err)
	return ok && typed.Layer == layer
}

// HTTPStatus maps business error code to transport status.
// Unknown/untyped errors are treated as internal server errors.
func HTTPStatus(err error) int {
	typed, ok := As(err)
	if !ok {
		return http.StatusInternalServerError
	}

	switch typed.Code {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeInvariantViolation:
		return http.StatusUnprocessableEntity
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodePermissionDenied:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
